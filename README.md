# romulusd

[![GoDoc](https://godoc.org/github.com/timelinelabs/romulus?status.svg)](https://godoc.org/github.com/timelinelabs/romulus)

Automagically register your kubernetes services in a loadbalancing proxy!

Supported loadbalancers: [vulcand](http://vulcand.github.io/), [traefik](http://traefik.github.io/)

Romulus works as an ingress controller in kubernetes. It listens for Ingress or Service additions / updates and registers the connected Endpoints in your backend loadbalancer provider. If you don't want to / can't use Ingress (because you're running an older kubernetes) then you can control resource routes with annotations in your Services.

## Usage

```
usage: romulusd [<flags>]

A kubernetes ingress controller

Flags:
  --help               Show context-sensitive help (also try --help-long and --help-man).
  -k, --kube-api=http://127.0.0.1:8080
                       URL for kubernetes api
  --kube-api-ver="v1"  kubernetes api version
  --kube-user=KUBE-USER
                       kubernetes username
  --kube-pass=KUBE-PASS
                       kubernetes password
  --kube-insecure      Run kubernetes client in insecure mode
  -s, --selector=label=value
                       label selectors. Leave blank for Everything(). Form: key=value
  -a, --annotations-prefix="romulus/"
                       annotations key prefix
  -p, --provider=vulcand
                       LoadBalancer provider
  --sync-interval=1h   Resync period with kube api
  --lb-timeout=10s     Timeout for communicating with loadbalancer provider
  --vulcan-api=http://127.0.0.1:8182
                       URL for vulcand api
  --traefik-etcd=TRAEFIK-ETCD
                       etcd peers for traefik
  -l, --log-level=info
                       log level. One of: fatal, error, warn, info, debug
```

If you are using Ingress, create your things as follows (assuming you set `--selector=route=public`):

```yaml
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: api-v1
  namespace: myapp
  labels:
    romulus/route: public
spec:
  rules:
  - host: www.example.com
    paths:
    - path: /v1
      backend:
        serviceName: microservice-v1
        servicePort: api
---
apiVersion: v1
kind: Service
metadata:
  name: microservice-v1
  namespace: myapp
  labels:
    romulus/route: public
spec:
  selector:
    app: myapi
    version: v1
  ports:
  - name: api
    port: 80
    targetPort: http
    protocol: TCP
```

If you do not use Ingresses:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: blog
  namespace: myblog
  annotations:
    romulus/host: 'www.example.com'
    romulus/prefix: '/blog'
    romulus/pass_host_header: true
    romulus/trust_forward_headers: true
  labels:
    romulus/route: public
spec:
  selector:
    app: blog
  ports:
  - name: web
    port: 80
    targetPort: http
    protocol: TCP
```

When you create these things, Romulus will turn around and upsert routes to the resulting Endpoints in your loadbalancer provider!

See [the docs](https://github.com/albertrdixon/romulus/wiki) and [the examples](./examples) for more info
