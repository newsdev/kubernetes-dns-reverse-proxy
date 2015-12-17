# Kubernetes DNS Reverse Proxy

A web server to route requests in a Kubernetes cluster to a corresponding Service based upon routes describes in a JSON configuration file.

Itself runs as a container within a kubernetes cluster with the path to the configuration file and options passed as command-line arguments.

### Options

`--domain-suffixes` Domain suffixes, comma separated. Default: `.local`

`--kubernetes-dns-domain` Kubernetes DNS domain. Default: `cluster.local`

`--kubernetes-namespace` Kubernetes namespace to server. Default: `default`

`--static` Whether to enable the proxy to serve content from a static file server. Default: `false`

`--static-scheme` Scheme of the static file server. Default: `http`

`--static-host` Hostname of the static file server. Default: ``

`--static-path` Path prefix of the static file server. Default: `/`

`--fallback` Whether to enable a fallback proxy. Default: `false`

`--fallback-scheme` Fallback scheme. Default: `http`

`--fallback-host` Fallback host. Default: ``

`--fallback-path` Fallback path. Default: `/`

`--routes` Absolute path to the routes file. Default: ``

`--concurrency` concurrency per host. Default: `32`

`--timeout` dial timeout.

### Routes Syntax

| pattern  | result   | example |
| -------- | -------- | ------- |
| service_name | Routed to service | "myservice" routed to myservice.<kubernetes-namespace>.<kubernetes-dns-domain> |
| /static_dir  | Routed to static-host + /static_dir + request_path | "/elections" routed to <static-host>/<static-path>/request_path |


### How to contribute

TK

### How to run the demo

```
git clone git@github.com:newsdev/kubernetes-routing.git
cd kubernetes-routing
./test/demo.sh
```

This boots a copy of the kubernetes-routing server on localhost:8080, and the test server on localhost:8090 (this echos back the `Host` heeader provided).

Hit [http://www.127.0.0.1.xip.io:8080/projects/app1](http://www.127.0.0.1.xip.io:8080/projects/app1) and check your local logs.  You'll see this route to service1 as specified in the routes.  

Try [http://www.127.0.0.1.xip.io:8080/projects/app2](http://www.127.0.0.1.xip.io:8080/projects/app2).  This should route to service 2.


#### How to run test suite

TK

#### Performance benchmarking

```
make benchmark
```

