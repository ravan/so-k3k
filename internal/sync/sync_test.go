package sync

import (
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"github.com/ravan/so-k3k/internal/config"
	"github.com/ravan/stackstate-client/stackstate/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestSync(t *testing.T) {
	//Dump the http requests to server if you need to debug.
	api.DumpHttpRequest = false

	// Use the following config for connecting to live server
	//conf := getConfig(t)

	//Mock Server
	conf, server := getMockConf(t)
	defer server.Close()

	factory, err := Sync(conf)
	require.NoError(t, err)
	assert.Equal(t, 11, factory.GetComponentCount())
	assert.Equal(t, 6, factory.GetRelationCount())
	
}

// -- Testing Infrastructure --

var opts = &slog.HandlerOptions{Level: slog.LevelDebug}
var handler = slog.NewJSONHandler(os.Stdout, opts)
var logger = slog.New(handler)

func init() { slog.SetDefault(logger) }

func getMockConf(t *testing.T) (*config.Configuration, *httptest.Server) {
	conf := getConfig(t)
	server := getMockServer(conf, getMockHandler(t))
	return conf, server
}

func getMockServer(conf *config.Configuration, hf http.HandlerFunc) *httptest.Server {
	server := httptest.NewServer(hf)
	conf.SuseObservability.ApiUrl = server.URL
	return server
}

func getMockHandler(t *testing.T) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/snapshot":
			queryReq := api.ViewSnapshotRequest{}
			err := json.NewDecoder(r.Body).Decode(&queryReq)
			require.NoError(t, err)
			switch queryReq.Query {
			case "type = 'pod' and label = 'role:server'":
				loadRespFile(w, "api/snapshot/k3k_server_query_resp.json")
			case "type = 'node' AND label in ('cluster-name:vcluster', 'cluster-name:mycluster')":
				loadRespFile(w, "api/snapshot/k3k_server_nodes_query_resp.json")
			case "(type = 'pod' and label = 'cluster-name:vcluster' and label = 'namespace:layer3' and label = 'k3k.io/clusterName:mycluster') OR (type = 'pod' and label = 'cluster-name:mycluster')":
				loadRespFile(w, "api/snapshot/k3k_shared_pods_query_resp.json")
			case "type = 'pod' and label = 'type:agent' and label = 'mode:shared' and label = 'cluster-name:vcluster' and label = 'namespace:layer3' and label = 'cluster:mycluster'":
				loadRespFile(w, "api/snapshot/k3k_virtual_node_pod_query_resp.json")
			default:
				assert.Fail(t, fmt.Sprintf("unexpected queeeeery: %s", queryReq.Query))
			}
		default:
			assert.Fail(t, fmt.Sprintf("unexpected request: %s", r.URL.Path))
		}
	}
}

func loadRespFile(w http.ResponseWriter, path string) {
	path = fmt.Sprintf("../../testdata/%s", path)
	_, err := os.Stat(path)
	if err == nil {
		file, err := os.ReadFile(path)
		if err == nil {
			_, err := w.Write(file)
			if err == nil {
				return
			}
		}
	}
	slog.Error("file not found", "path", path)
	w.WriteHeader(http.StatusNotFound)
}

func getConfig(t *testing.T) *config.Configuration {
	require.NoError(t, os.Setenv("CONFIG_FILE", "../../conf.yaml"))
	require.NoError(t, godotenv.Load("../../.env"))
	require.NoError(t, os.Setenv("suseobservability.api_url", os.Getenv("SO_URL")))
	require.NoError(t, os.Setenv("suseobservability.api_key", os.Getenv("SO_API_KEY")))
	require.NoError(t, os.Setenv("suseobservability.api_token", os.Getenv("SO_TOKEN")))

	c, err := config.GetConfig()
	require.NoError(t, err)
	return c
}
