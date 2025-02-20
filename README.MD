# SUSE Observability Extension for K3K

[SUSE Observability](https://www.suse.com/solutions/observability/) is full-stack observability platform with tons of 
extension and integration possibilities.

This repository contains the SUSE Observability extension to support [K3K](https://github.com/rancher/k3k).

K3K, Kubernetes in Kubernetes, is a tool that empowers you to create and manage isolated K3s clusters within your
existing Kubernetes environment.

## SUSE Observability Agent 

Out of the box SUSE Observability's agent can observe **K3K** clusters with slight tweaks.

![k3k topo from agent](./setup/img/topo_with_agents.png)

Follow the [default instructions](https://docs.stackstate.com/get-started/k8s-suse-rancher-prime#installing-the-suse-observability-agent)
to install the agent. 

In **Virtual Mode**, the Process Agent Network Trace Off needs to be turned off because the current **K3K** image
does not have an OS with eBPF turned on. In the virtual cluster's kubeconfig context run,

```bash
kubectl set env daemonset/suse-observability-agent-node-agent -n monitoring STS_NETWORK_TRACING_ENABLED=false
```
In **Shared Mode**, there is no actual nodes. Monitoring logs and nodes happen in the host cluster. So we can remove
the agent's daemonset for these. In the shared cluster's kubeconfig context run,

```bash
- kubectl delete daemonset suse-observability-agent-node-agent -n monitoring 
- kubectl delete daemonset suse-observability-agent-logs-agent -n monitoring
```

## Extension Design

The extension builds on the existing Kubernetes data in SUSE Observability. It creates relations and adjusts `Layer` 
properties to better map the K3K domain concepts.

![extension logic](./setup/img/extension_logic.png)

Once installed the extension will send data to an Autosync StackPack instance and SUSE Observability will merge
this data with the default Kubernetes data, creating a more complete topology.

![extension logic result](./setup/img/extension_logic_result.png)


See [Setup Guide](./setup/README.MD) for a comprehensive example of setting up an environment and deploying
the extension.

#### Custom Menu

![menu](./setup/img/menu.png)

#### k3k Servers Overview Page

![menu](./setup/img/servers.png)

#### k3k Shared Mode Pods Overview Page

![shared pods](./setup/img/shared_pods.png)

## Helm Deployment

Setup a basic `values.yaml` containing connectivity information about your SUSE Observability instance.

```yaml
serverUrl:  https://xxxx.     # Suse Observability Url
apiKey: xxx                   # Suse Observability Api Key
apiToken: xxx                 # Suse Observability CLI api token.
instanceName: lab             # Instance name as defined for the AutoSync StackPack instance in Suse Observability
```

Then run the following Helm commands,

```bash
helm repo add so-ext https://ravan.github.io/helm-charts
helm upgrade --install --namespace suse-observability-extensions --create-namespace -f values.yaml so-k3k suse-observability-addons/so-k3k                            

```

## Configuration

### Environment variables

Environment variables can be used to set up the extension.

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
suseobservability:
  api_url: "https://xxx.stackstate.io"
  api_key: "5385xxxx"
  api_token: "5xxxx"
  
instance:
  type: k3k
  url: inception

```



## Development

### Prerequisites

- [Taskfile](https://taskfile.dev/installation/)


### Run

Setup the `conf.yaml' file with your environment details.

```bash
cp conf.yaml.example conf.yaml
task run
```

### Build

Add the Docker login info to your `.env` file.

```
USER="<my dockerhub user name>"
PWD="<my dockerhub password>"
```

To build and push the container, 

```shell
task docker-build
task docker-push
```

