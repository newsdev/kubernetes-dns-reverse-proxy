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

TK

### How to contribute

TK

### How to build

The Makefile builds within a docker container so you must have docker set-up locally.

#### How to run test suite

TK

#### Performance benchmarking

```
make benchmark
```

