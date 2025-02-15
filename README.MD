# SUSE Observability Extension for K3K

[SUSE Observability](https://www.suse.com/solutions/observability/) is full-stack observability platform with tons of extension and integration possibilities.

Out of the box SUSE Observability's agent can observe [k3k](https://github.com/rancher/k3k) clusters.

This extension will link the nodes of a nested cluster with the supporting pod in the outer cluster.


## Helm Deployment

Setup a basic values.yaml containing connectivity information about your SUSE Observability instance.

```yaml
serverUrl:  https://xxxx.    # Suse Observability Url
apiKey: xxx                  # Suse Observability Api Key
clusterName: lab             # Cluster name as defined for the Kubernetes StackPack instance in Suse Observability
```

Then run the following Helm commands,

```bash
helm repo add so-ext https://ravan.github.io/helm-charts
helm upgrade --install --namespace stackstate-extensions --create-namespace -f values.yaml so so-ext/suse-observabilty-k3k-ext                            

```

## Configuration

### Environment variables

Environment variables can be used to set up the scanner.

| Variable                       | Required | Default        | Description                                              |
|--------------------------------|----------|----------------|----------------------------------------------------------|
| SUSEOBSERVABILITY_API_URL      | yes      |                | The url to the SUSE Observability server                 |
| SUSEOBSERVABILITY_API_KEY      | yes      |                | The api key used by the SUSE Observability server        |
| SUSEOBSERVABILITY_API_TOKEN    | yes      |                | The cli token used to access SUSE Observability API      |
| INSTANCE_TYPE                  | no       | k3k            | The Custom Syn StackPack instance type.                  |
| INSTANCE_URL                   | yes      |                | The Custom Syn StackPack instance url                    |


### Config File

Configuration could also be in a yaml file.
The location of the config file can be set in the `CONFIG_FILE`environment variable

```yaml
so:
  api_url: "https://xxx.stackstate.io"
  api_key: "5385xxxx"
  
instance:
  type: k3k
  url: inception

```


## Development

### Prerequisites

- [Taskfile](https://taskfile.dev/installation/)


### Setup environment variables

Set up a `.env` file for configuring the env.

```
USER="<my dockerhub user name>"
PWD="<my dockerhub password>"
SUSEOBSERVABILITY_URL=https://myinstance.stackstate.io
SUSEOBSERVABILITY_API_KEY=xxxx
SUSEOBSERVABILITY_TOKEN_KEY=xxxx
```
### Build

```shell
task docker-build
```

### Push

```shell
task docker-push
```

