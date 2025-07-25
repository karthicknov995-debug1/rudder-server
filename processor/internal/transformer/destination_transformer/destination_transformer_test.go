package destination_transformer_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	transformertest "github.com/rudderlabs/rudder-go-kit/testhelper/docker/resource/transformer"

	"github.com/rudderlabs/rudder-go-kit/config"
	"github.com/rudderlabs/rudder-go-kit/jsonrs"
	"github.com/rudderlabs/rudder-go-kit/logger"
	"github.com/rudderlabs/rudder-go-kit/logger/mock_logger"
	"github.com/rudderlabs/rudder-go-kit/stats"
	"github.com/rudderlabs/rudder-go-kit/stats/memstats"
	"github.com/rudderlabs/rudder-go-kit/testhelper/rand"

	backendconfig "github.com/rudderlabs/rudder-server/backend-config"
	"github.com/rudderlabs/rudder-server/gateway/response"
	transformerutils "github.com/rudderlabs/rudder-server/processor/internal/transformer"
	"github.com/rudderlabs/rudder-server/processor/internal/transformer/destination_transformer"
	"github.com/rudderlabs/rudder-server/processor/types"
	"github.com/rudderlabs/rudder-server/testhelper/backendconfigtest"
	reportingtypes "github.com/rudderlabs/rudder-server/utils/types"
	warehouseutils "github.com/rudderlabs/rudder-server/warehouse/utils"
)

type fakeTransformer struct {
	requests [][]types.TransformerEvent
	t        testing.TB
}

func (t *fakeTransformer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var reqBody []types.TransformerEvent
	if r.Header.Get("X-Content-Format") == "json+compactedv1" {
		var ctr types.CompactedTransformRequest
		require.NoError(t.t, jsonrs.NewDecoder(r.Body).Decode(&ctr))
		reqBody = ctr.ToTransformerEvents()
	} else {
		require.NoError(t.t, jsonrs.NewDecoder(r.Body).Decode(&reqBody))
	}

	t.requests = append(t.requests, reqBody)

	responses := make([]types.TransformerResponse, len(reqBody))

	for i := range reqBody {
		statusCode := int(reqBody[i].Message["forceStatusCode"].(float64))
		delete(reqBody[i].Message, "forceStatusCode")
		reqBody[i].Message["echo-key-1"] = reqBody[i].Message["src-key-1"]

		responses[i] = types.TransformerResponse{
			Output:     reqBody[i].Message,
			Metadata:   reqBody[i].Metadata,
			StatusCode: statusCode,
		}
		if statusCode >= http.StatusBadRequest {
			responses[i].Error = "error"
		}
	}

	w.Header().Set("apiVersion", strconv.Itoa(reportingtypes.SupportedTransformerApiVersion))

	require.NoError(t.t, jsonrs.NewEncoder(w).Encode(responses))
}

type endlessLoopTransformer struct {
	retryCount    int
	maxRetryCount int

	skipApiVersion bool
	apiVersion     int

	statusCode  int
	statusError string

	t testing.TB
}

func (elt *endlessLoopTransformer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	elt.retryCount++

	var reqBody []types.TransformerEvent
	if r.Header.Get("X-Content-Format") == "json+compactedv1" {
		var ctr types.CompactedTransformRequest
		require.NoError(elt.t, jsonrs.NewDecoder(r.Body).Decode(&ctr))
		reqBody = ctr.ToTransformerEvents()
	} else {
		require.NoError(elt.t, jsonrs.NewDecoder(r.Body).Decode(&reqBody))
	}

	responses := make([]types.TransformerResponse, len(reqBody))

	if elt.retryCount < elt.maxRetryCount {
		http.Error(w, response.MakeResponse(elt.statusError), elt.statusCode)
		return
	}

	for i := range reqBody {
		responses[i] = types.TransformerResponse{
			Output:     reqBody[i].Message,
			Metadata:   reqBody[i].Metadata,
			StatusCode: http.StatusOK,
			Error:      "",
		}
	}

	if !elt.skipApiVersion {
		w.Header().Set("apiVersion", strconv.Itoa(elt.apiVersion))
	}

	require.NoError(elt.t, jsonrs.NewEncoder(w).Encode(responses))
}

type endpointTransformer struct {
	t              testing.TB
	supportedPaths []string
}

func (et *endpointTransformer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	if !slices.Contains(et.supportedPaths, path) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	var reqBody []types.TransformerEvent
	if r.Header.Get("X-Content-Format") == "json+compactedv1" {
		var ctr types.CompactedTransformRequest
		require.NoError(et.t, jsonrs.NewDecoder(r.Body).Decode(&ctr))
		reqBody = ctr.ToTransformerEvents()
	} else {
		require.NoError(et.t, jsonrs.NewDecoder(r.Body).Decode(&reqBody))
	}

	responses := make([]types.TransformerResponse, len(reqBody))

	for i := range reqBody {
		responses[i] = types.TransformerResponse{
			Output:     reqBody[i].Message,
			Metadata:   reqBody[i].Metadata,
			StatusCode: http.StatusOK,
			Error:      "",
		}
	}

	w.Header().Set("apiVersion", strconv.Itoa(reportingtypes.SupportedTransformerApiVersion))

	require.NoError(et.t, jsonrs.NewEncoder(w).Encode(responses))
}

func TestDestinationTransformer(t *testing.T) {
	clientTypes := []string{"stdlib", "recycled", "httplb"}
	for _, clientType := range clientTypes {
		t.Run(fmt.Sprintf("with %s client", clientType), func(t *testing.T) {
			conf := config.New()
			conf.Set("Transformer.Client.type", clientType)

			t.Run("success", func(t *testing.T) {
				ft := &fakeTransformer{
					t: t,
				}

				srv := httptest.NewServer(ft)
				defer srv.Close()

				tc := []struct {
					batchSize   int
					eventsCount int
					failEvery   int
				}{
					{batchSize: 0, eventsCount: 0},
					{batchSize: 10, eventsCount: 100},
					{batchSize: 10, eventsCount: 9},
					{batchSize: 10, eventsCount: 91},
					{batchSize: 10, eventsCount: 99},
					{batchSize: 10, eventsCount: 1},
					{batchSize: 10, eventsCount: 80, failEvery: 4},
					{batchSize: 10, eventsCount: 80, failEvery: 1},
				}

				for _, tt := range tc {
					statsStore, err := memstats.New()
					require.NoError(t, err)

					conf.Set("Processor.Transformer.failOnUserTransformTimeout", true)
					conf.Set("Processor.Transformer.failOnError", true)
					conf.Set("Processor.Transformer.maxRetryBackoffInterval", 1*time.Second)
					conf.Set("Processor.Transformer.maxRetry", 1)
					conf.Set("Processor.Transformer.timeoutDuration", 1*time.Second)
					conf.Set("DEST_TRANSFORM_URL", srv.URL)
					conf.Set("Processor.transformBatchSize", tt.batchSize)
					tr := destination_transformer.New(conf, logger.NOP, statsStore, destination_transformer.WithClient(srv.Client()))
					eventsCount := tt.eventsCount
					failEvery := tt.failEvery

					events := make([]types.TransformerEvent, eventsCount)
					expectedResponse := types.Response{}

					transformationID := rand.String(10)

					destinationConfig := backendconfigtest.NewDestinationBuilder("WEBHOOK").
						WithUserTransformation(transformationID, rand.String(10)).Build()

					Metadata := types.Metadata{
						DestinationType:  destinationConfig.DestinationDefinition.Name,
						SourceID:         rand.String(10),
						DestinationID:    destinationConfig.ID,
						TransformationID: destinationConfig.Transformations[0].ID,
					}

					for i := range events {
						msgID := fmt.Sprintf("messageID-%d", i)
						statusCode := http.StatusOK

						if failEvery != 0 && i%failEvery == 0 {
							statusCode = http.StatusBadRequest
						}

						Metadata := Metadata
						Metadata.MessageID = msgID

						events[i] = types.TransformerEvent{
							Metadata: Metadata,
							Message: map[string]interface{}{
								"src-key-1":       msgID,
								"forceStatusCode": statusCode,
							},
							Destination: destinationConfig,
							Credentials: []types.Credential{
								{
									ID:       "test-credential",
									Key:      "test-key",
									Value:    "test-value",
									IsSecret: false,
								},
							},
						}

						tResp := types.TransformerResponse{
							Metadata:   Metadata,
							StatusCode: statusCode,
							Output: map[string]interface{}{
								"src-key-1":  msgID,
								"echo-key-1": msgID,
							},
						}

						if statusCode < http.StatusBadRequest {
							expectedResponse.Events = append(expectedResponse.Events, tResp)
						} else {
							tResp.Error = "error"
							expectedResponse.FailedEvents = append(expectedResponse.FailedEvents, tResp)
						}
					}

					labels := types.TransformerMetricLabels{
						Endpoint:        transformerutils.GetEndpointFromURL(srv.URL),
						Stage:           "dest_transformer",
						SourceID:        Metadata.SourceID,
						SourceType:      Metadata.SourceType,
						DestinationType: destinationConfig.DestinationDefinition.Name,
						DestinationID:   destinationConfig.ID,
						WorkspaceID:     Metadata.WorkspaceID,
					}
					rsp := tr.Transform(context.TODO(), events)
					require.Equal(t, expectedResponse, rsp)

					metrics := statsStore.GetByName("processor_transformer_request_time")
					if tt.eventsCount > 0 {
						require.NotEmpty(t, metrics)
						for _, m := range metrics {
							require.Equal(t, stats.Tags{
								"endpoint":         transformerutils.GetEndpointFromURL(srv.URL),
								"stage":            "dest_transformer",
								"sourceId":         Metadata.SourceID,
								"sourceType":       Metadata.SourceType,
								"destinationType":  destinationConfig.DestinationDefinition.Name,
								"destinationId":    destinationConfig.ID,
								"workspaceId":      Metadata.WorkspaceID,
								"language":         "",
								"transformationId": "",
								"mirroring":        "false",

								// Legacy tags: to be removed
								"dest_type": destinationConfig.DestinationDefinition.Name,
								"dest_id":   destinationConfig.ID,
								"src_id":    Metadata.SourceID,
							}, m.Tags)

							metricsToCheck := []string{
								"transformer_client_request_total_bytes",
								"transformer_client_response_total_bytes",
								"transformer_client_request_total_events",
								"transformer_client_response_total_events",
								"transformer_client_total_durations_seconds",
							}

							expectedTags := labels.ToStatsTag()
							for _, metricName := range metricsToCheck {
								measurements := statsStore.GetByName(metricName)
								require.NotEmpty(t, measurements, "metric %s should not be empty", metricName)
								require.Equal(t, expectedTags, measurements[0].Tags, "metric %s tags mismatch", metricName)
							}
						}
					}
				}
			})

			t.Run("timeout", func(t *testing.T) {
				msgID := "messageID-0"
				events := append([]types.TransformerEvent{}, types.TransformerEvent{
					Metadata: types.Metadata{
						MessageID: msgID,
					},
					Message: map[string]interface{}{
						"src-key-1": msgID,
					},
					Credentials: []types.Credential{
						{
							ID:       "test-credential",
							Key:      "test-key",
							Value:    "test-value",
							IsSecret: false,
						},
					},
				})

				testCases := []struct {
					name                       string
					retries                    int
					failOnUserTransformTimeout bool
					expectPanic                bool
				}{
					{
						name:                       "destination transformation timeout",
						retries:                    3,
						failOnUserTransformTimeout: false,
						expectPanic:                true,
					},
					{
						name:                       "destination transformation timeout with fail on timeout",
						retries:                    3,
						failOnUserTransformTimeout: true,
						expectPanic:                false,
					},
				}

				for _, tc := range testCases {
					tc := tc

					t.Run(tc.name, func(t *testing.T) {
						ch := make(chan struct{})
						srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
							<-ch
						}))
						defer srv.Close()

						client := srv.Client()
						client.Timeout = 1 * time.Millisecond

						conf.Set("Processor.maxRetry", tc.retries)
						conf.Set("Processor.Transformer.failOnError", tc.failOnUserTransformTimeout)
						conf.Set("Processor.maxRetryBackoffInterval", 1*time.Second)
						conf.Set("DEST_TRANSFORM_URL", srv.URL)
						tr := destination_transformer.New(conf, logger.NOP, stats.Default, destination_transformer.WithClient(client))

						if tc.expectPanic {
							require.Panics(t, func() {
								_ = tr.Transform(context.TODO(), events)
							})
							close(ch)
							return
						}
					})
				}
			})

			t.Run("retries", func(t *testing.T) {
				msgID := "messageID-0"
				events := append([]types.TransformerEvent{}, types.TransformerEvent{
					Metadata: types.Metadata{
						MessageID: msgID,
					},
					Message: map[string]interface{}{
						"src-key-1": msgID,
					},
					Destination: backendconfig.DestinationT{
						Transformations: []backendconfig.TransformationT{
							{
								ID:        "test-transformation",
								VersionID: "test-version",
							},
						},
					},
					Credentials: []types.Credential{
						{
							ID:       "test-credential",
							Key:      "test-key",
							Value:    "test-value",
							IsSecret: false,
						},
					},
				})

				testCases := []struct {
					name             string
					retries          int
					maxRetryCount    int
					statusCode       int
					statusError      string
					expectedRetries  int
					expectPanic      bool
					expectedResponse types.Response
					failOnError      bool
				}{
					{
						name:            "too many requests",
						retries:         3,
						maxRetryCount:   10,
						statusCode:      http.StatusTooManyRequests,
						statusError:     "too many requests",
						expectedRetries: 4,
						expectPanic:     true,
						failOnError:     false,
					},
					{
						name:            "too many requests with fail on error",
						retries:         3,
						maxRetryCount:   10,
						statusCode:      http.StatusTooManyRequests,
						statusError:     "too many requests",
						expectedRetries: 4,
						expectPanic:     false,
						expectedResponse: types.Response{
							Events: nil,
							FailedEvents: []types.TransformerResponse{
								{
									Metadata: types.Metadata{
										MessageID: msgID,
									},
									StatusCode: transformerutils.TransformerRequestFailure,
									Error:      "transformer request failed: transformer returned status code: 429",
								},
							},
						},
						failOnError: true,
					},
				}

				for _, tc := range testCases {
					tc := tc
					t.Run(tc.name, func(t *testing.T) {
						elt := &endlessLoopTransformer{
							maxRetryCount: tc.maxRetryCount,
							statusCode:    tc.statusCode,
							statusError:   tc.statusError,
							apiVersion:    reportingtypes.SupportedTransformerApiVersion,
							t:             t,
						}

						srv := httptest.NewServer(elt)
						defer srv.Close()

						conf.Set("DEST_TRANSFORM_URL", srv.URL)
						conf.Set("Processor.maxRetry", tc.retries)
						conf.Set("Processor.maxRetryBackoffInterval", 1*time.Second)
						conf.Set("Processor.timeoutDuration", 1*time.Second)
						conf.Set("Processor.Transformer.failOnUserTransformTimeout", false)
						conf.Set("Processor.Transformer.failOnError", tc.failOnError)

						tr := destination_transformer.New(conf, logger.NOP, stats.Default, destination_transformer.WithClient(srv.Client()))

						if tc.expectPanic {
							require.Panics(t, func() {
								_ = tr.Transform(context.TODO(), events)
							})
							require.Equal(t, elt.retryCount, tc.expectedRetries)
							return
						}

						rsp := tr.Transform(context.TODO(), events)
						require.Equal(t, tc.expectedResponse, rsp)
						require.Equal(t, tc.expectedRetries, elt.retryCount)
					})
				}
			})

			t.Run("version compatibility", func(t *testing.T) {
				msgID := "messageID-0"
				events := append([]types.TransformerEvent{}, types.TransformerEvent{
					Metadata: types.Metadata{
						MessageID: msgID,
					},
					Message: map[string]interface{}{
						"src-key-1": msgID,
					},
					Destination: backendconfig.DestinationT{
						Transformations: []backendconfig.TransformationT{
							{
								ID:        "test-transformation",
								VersionID: "test-version",
							},
						},
					},
					Credentials: []types.Credential{
						{
							ID:       "test-credential",
							Key:      "test-key",
							Value:    "test-value",
							IsSecret: false,
						},
					},
				})

				testCases := []struct {
					name             string
					apiVersion       int
					skipApiVersion   bool
					expectPanic      bool
					expectedResponse types.Response
				}{
					{
						name:        "compatible api version",
						apiVersion:  reportingtypes.SupportedTransformerApiVersion,
						expectPanic: false,
						expectedResponse: types.Response{
							FailedEvents: nil,
							Events: []types.TransformerResponse{
								{
									Metadata: types.Metadata{
										MessageID: msgID,
									},
									StatusCode: http.StatusOK,
									Output: map[string]interface{}{
										"src-key-1": msgID,
									},
								},
							},
						},
					},
					{
						name:        "incompatible api version",
						apiVersion:  1,
						expectPanic: true,
					},
					{
						name:           "unexpected api version",
						skipApiVersion: true,
						expectPanic:    true,
					},
				}

				for _, tc := range testCases {
					tc := tc

					t.Run(tc.name, func(t *testing.T) {
						elt := &endlessLoopTransformer{
							maxRetryCount:  0,
							skipApiVersion: tc.skipApiVersion,
							apiVersion:     tc.apiVersion,
							t:              t,
						}

						srv := httptest.NewServer(elt)
						defer srv.Close()

						conf.Set("DEST_TRANSFORM_URL", srv.URL)
						conf.Set("Processor.maxRetry", 1)
						conf.Set("Processor.maxRetryBackoffInterval", 1*time.Second)
						conf.Set("Processor.timeoutDuration", 1*time.Second)
						tr := destination_transformer.New(conf, logger.NOP, stats.Default, destination_transformer.WithClient(srv.Client()))
						if tc.expectPanic {
							require.Panics(t, func() {
								_ = tr.Transform(context.TODO(), events)
							})
							return
						}

						rsp := tr.Transform(context.TODO(), events)
						require.Equal(t, tc.expectedResponse, rsp)
					})
				}
			})

			t.Run("endpoints", func(t *testing.T) {
				msgID := "messageID-0"
				expectedResponse := types.Response{
					Events: []types.TransformerResponse{
						{
							Output: map[string]interface{}{
								"src-key-1": msgID,
							},
							Metadata: types.Metadata{
								MessageID: msgID,
							},
							StatusCode: http.StatusOK,
						},
					},
				}
				events := append([]types.TransformerEvent{}, types.TransformerEvent{
					Metadata: types.Metadata{
						MessageID: msgID,
					},
					Message: map[string]interface{}{
						"src-key-1": msgID,
					},
					Destination: backendconfig.DestinationT{
						DestinationDefinition: backendconfig.DestinationDefinitionT{
							Name: "test-destination",
						},
						Transformations: []backendconfig.TransformationT{
							{
								ID:        "test-transformation",
								VersionID: "test-version",
							},
						},
					},
					Credentials: []types.Credential{
						{
							ID:       "test-credential",
							Key:      "test-key",
							Value:    "test-value",
							IsSecret: false,
						},
					},
				})

				t.Run("Destination transformations", func(t *testing.T) {
					et := &endpointTransformer{
						supportedPaths: []string{"/v0/destinations/test-destination"},
						t:              t,
					}

					srv := httptest.NewServer(et)
					defer srv.Close()

					c := config.New()
					c.Set("Processor.maxRetry", 1)
					c.Set("DEST_TRANSFORM_URL", srv.URL)

					tr := destination_transformer.New(c, logger.NOP, stats.Default, destination_transformer.WithClient(srv.Client()))
					rsp := tr.Transform(context.TODO(), events)
					require.Equal(t, rsp, expectedResponse)
				})

				t.Run("Destination warehouse transformations", func(t *testing.T) {
					testCases := []struct {
						name            string
						destinationType string
					}{
						{
							name:            "rs",
							destinationType: warehouseutils.RS,
						},
						{
							name:            "clickhouse",
							destinationType: warehouseutils.CLICKHOUSE,
						},
						{
							name:            "snowflake",
							destinationType: warehouseutils.SNOWFLAKE,
						},
					}

					for _, tc := range testCases {
						tc := tc

						t.Run(tc.name, func(t *testing.T) {
							et := &endpointTransformer{
								supportedPaths: []string{`/v0/destinations/` + tc.name},
								t:              t,
							}

							srv := httptest.NewServer(et)
							defer srv.Close()

							c := config.New()
							c.Set("Processor.maxRetry", 1)
							c.Set("DEST_TRANSFORM_URL", srv.URL)

							tr := destination_transformer.New(c, logger.NOP, stats.Default, destination_transformer.WithClient(srv.Client()))

							events := append([]types.TransformerEvent{}, types.TransformerEvent{
								Metadata: types.Metadata{
									MessageID: msgID,
								},
								Message: map[string]interface{}{
									"src-key-1": msgID,
								},
								Destination: backendconfig.DestinationT{
									DestinationDefinition: backendconfig.DestinationDefinitionT{
										Name: tc.destinationType,
									},
									Transformations: []backendconfig.TransformationT{
										{
											ID:        "test-transformation",
											VersionID: "test-version",
										},
									},
								},
								Credentials: []types.Credential{
									{
										ID:       "test-credential",
										Key:      "test-key",
										Value:    "test-value",
										IsSecret: false,
									},
								},
							})

							rsp := tr.Transform(context.TODO(), events)
							require.Equal(t, rsp, expectedResponse)
						})
					}
				})
			})
		})
	}
}

func TestLongRunningTransformation(t *testing.T) {
	fileName := t.TempDir() + "out.log"
	f, err := os.Create(fileName)
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	ctrl := gomock.NewController(t)

	t.Run("context cancels before timeout", func(t *testing.T) {
		mockLogger := mock_logger.NewMockLogger(ctrl)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		transformerutils.TrackLongRunningTransformation(ctx, "stage", time.Hour, mockLogger)
	})

	t.Run("log stmt", func(t *testing.T) {
		mockLogger := mock_logger.NewMockLogger(ctrl)
		var fired atomic.Bool
		mockLogger.EXPECT().Errorw(gomock.Any(), gomock.Any()).Do(func(msg string, keysAndValues ...interface{}) {
			require.Equal(t, "Long running transformation detected", msg)
			require.Len(t, keysAndValues, 4)
			require.Equal(t, "stage", keysAndValues[0])
			require.Equal(t, "stage", keysAndValues[1])
			require.Equal(t, "duration", keysAndValues[2])
			_, err := time.ParseDuration(keysAndValues[3].(string))
			require.NoError(t, err)
			fired.Store(true)
		}).MinTimes(1)
		ctx, cancel := context.WithCancel(context.Background())
		go transformerutils.TrackLongRunningTransformation(ctx, "stage", time.Millisecond, mockLogger)
		for !fired.Load() {
			time.Sleep(time.Millisecond)
		}
		cancel()
	})
}

func TestEmbeddedWarehouseTransformer(t *testing.T) {
	pool, err := dockertest.NewPool("")
	require.NoError(t, err)
	transformerResource, err := transformertest.Setup(pool, t)
	require.NoError(t, err)

	conf := config.New()
	conf.Set("DEST_TRANSFORM_URL", transformerResource.TransformerURL)
	conf.Set("USER_TRANSFORM_URL", transformerResource.TransformerURL)
	conf.Set("Processor.enableWarehouseTransformations", true)
	conf.Set("Processor.verifyWarehouseTransformations", true)

	ctx := context.Background()
	eventsCount := 10000
	trans := destination_transformer.New(conf, logger.NOP, stats.NOP)

	clientEvents := lo.RepeatBy(eventsCount, func(index int) types.TransformerEvent {
		return types.TransformerEvent{
			Message: map[string]interface{}{
				"src-key-1":         "test-value-1",
				"messageId":         "messageId" + strconv.Itoa(index+1),
				"event":             "event" + strconv.Itoa(index+1),
				"originalTimestamp": "2021-09-01T00:00:00.000Z",
				"receivedAt":        "2021-09-01T00:00:00.000Z",
				"sentAt":            "2021-09-01T00:00:00.000Z",
				"timestamp":         "2021-09-01T00:00:00.000Z",
				"type":              "track",
			},
			Metadata: types.Metadata{
				MessageID:       "messageId" + strconv.Itoa(index+1),
				ReceivedAt:      "2021-09-01T00:00:00.000Z",
				SourceID:        "test-source",
				SourceType:      "JavaScript",
				DestinationID:   "test-destination",
				DestinationType: "RS",
				SourceCategory:  "webhook",
				EventType:       "track",
				RecordID:        "test-record",
				JobID:           int64(index + 1),
			},
			Destination: backendconfig.DestinationT{
				ID: "test-destination",
				DestinationDefinition: backendconfig.DestinationDefinitionT{
					Name: "RS",
				},
				Config: map[string]any{
					"skipTracksTable": true,
				},
			},
		}
	})
	expectedResponse := lo.Flatten(lo.RepeatBy(eventsCount, func(index int) []types.TransformerResponse {
		return []types.TransformerResponse{
			{
				Output: map[string]any{
					"data": map[string]any{
						"context_destination_id":   "test-destination",
						"context_destination_type": "RS",
						"context_source_id":        "test-source",
						"context_source_type":      "JavaScript",
						"event":                    "event" + strconv.Itoa(index+1),
						"event_text":               "event" + strconv.Itoa(index+1),
						"id":                       "messageId" + strconv.Itoa(index+1),
						"original_timestamp":       "2021-09-01T00:00:00.000Z",
						"received_at":              "2021-09-01T00:00:00.000Z",
						"sent_at":                  "2021-09-01T00:00:00.000Z",
						"timestamp":                "2021-09-01T00:00:00.000Z",
					},
					"metadata": map[string]any{
						"columns": map[string]any{
							"context_destination_id":   "string",
							"context_destination_type": "string",
							"context_source_id":        "string",
							"context_source_type":      "string",
							"event":                    "string",
							"event_text":               "string",
							"id":                       "string",
							"original_timestamp":       "datetime",
							"received_at":              "datetime",
							"sent_at":                  "datetime",
							"timestamp":                "datetime",
							"uuid_ts":                  "datetime",
						},
						"receivedAt": "2021-09-01T00:00:00.000Z",
						"table":      "event" + strconv.Itoa(index+1),
					},
					"userId": "",
				},
				Metadata: types.Metadata{
					SourceID:        "test-source",
					SourceType:      "JavaScript",
					SourceCategory:  "webhook",
					DestinationID:   "test-destination",
					RecordID:        "test-record",
					DestinationType: "RS",
					MessageID:       "messageId" + strconv.Itoa(index+1),
					ReceivedAt:      "2021-09-01T00:00:00.000Z",
					EventType:       "track",
					JobID:           int64(index + 1),
				},
				StatusCode: 200,
			},
		}
	}))

	r := trans.Transform(ctx, clientEvents)
	require.Empty(t, r.FailedEvents)
	require.Len(t, r.Events, eventsCount)

	expectedMessageIDs := lo.Flatten(lo.RepeatBy(eventsCount, func(index int) []string {
		return []string{"messageId" + strconv.Itoa(index+1)}
	}))
	actualMessageIDS := lo.Map(r.Events, func(item types.TransformerResponse, index int) string {
		return item.Metadata.MessageID
	})
	require.ElementsMatch(t, expectedMessageIDs, actualMessageIDS)
	require.ElementsMatch(t, expectedResponse, r.Events)
}
