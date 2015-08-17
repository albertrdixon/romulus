# romulusd

[![GoDoc](https://godoc.org/github.com/timelinelabs/romulus?status.svg)](https://godoc.org/github.com/timelinelabs/romulus)

Automagically register your kubernetes services in vulcan proxy!

## Usage

```
$ romulusd --help
usage: romulusd [<flags>]

Flags:
  --help           Show help (also see --help-long and --help-man).
  -v, --vulcand-key="vulcand"
                   vulcand etcd key
  -e, --etcd=http://127.0.0.1:2379
                   etcd peers
  -k, --kube=http://127.0.0.1:8080
                   kubernetes endpoint
  -U, --kube-user=KUBE-USER
                   kubernetes username
  -P, --kube-pass=KUBE-PASS
                   kubernetes password
  --kube-api="v1"  kubernetes api version
  -C, --kubecfg=/path/to/.kubecfg
                   path to kubernetes cfg file
  -s, --svc-selector=key=value[,key=value]
                   service selectors. Leave blank for Everything(). Form: key=value
  -l, --log-level=info
                   log level. One of: fatal, error, warn, info, debug
  --version        Show application version.
```

Set up your kubernetes service with a label and some options annotations:

*NOTE*: all labels and annotations are under the prefix `romulus/`

```yaml
apiVersion: v1
kind: Service
metadata:
  name: example
  annotations:
    romulus/host: 'www.example.com'
    romulus/path: '/guestbook'
    romulus/frontendSettings: '{"FailoverPredicate":"(IsNetworkError() || ResponseCode() == 503) && Attempts() <= 2"}}'
    romulus/backendSettings: '{"KeepAlive": {"MaxIdleConnsPerHost": 128, "Period": "4s"}}'
  labels:
    name: example
    romulus/type: external # <-- Will ensure SVC-SELECTORs specified (e.g. 'type=external') are present in either Labels or Annotations.
spec: 
...
```

When you create the service, romulusd will create keys in etcd for vulcan!

*NOTE*: IDs for backends and frontends are constructed as follows: `<kube resource name>[.<port name>].<namespace>`

```
$ kubectl.sh get svc,endpoints -l romulus/type=external
NAME           LABELS                            SELECTOR            IP(S)           PORT(S)
frontend       name=frontend,type=external       name=frontend       10.247.242.50   80/TCP
NAME           ENDPOINTS
frontend       10.246.1.7:80,10.246.1.8:80,10.246.1.9:80

$ etcdctl get /vulcan/backends/example.default/backend
{"Type":"http","Settings":{"KeepAlive":{"MaxIdleConnsPerHost":128,"Period": "4s"}}}

$ etcdctl get /vulcan/frontends/example.default/frontend
{"Type":"http","BackendId":"example.default","Route":"Host(`www.example.com`) && Path(`/guestbook`)","Settings":{"FailoverPredicate":"(IsNetworkError() || ResponseCode() == 503) && Attempts() <= 2"}}

$ etcdctl ls /vulcan/backends/example.default/servers
/vulcan/backends/example.default/servers/10.246.1.8
/vulcan/backends/example.default/servers/10.246.1.9
/vulcan/backends/example.default/servers/10.246.1.7
```

## Multi Port Services

If your service has multiple ports, romulusd will create a frontend for each.

Separate options by putting them under the prefix `romulus.<port name>/`. If no matching `romulus.<port name>/` option exists, then the `romulus/` option will be used.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: example
  annotations:
    romulus/host: 'www.example.com'
    romulus/path.api: '/api'
    romulus/path.web: '/web'
  labels:
    name: example
    romulus/type: external
spec:
  ports:
  - port: 80
    name: web
  - port: 8888
    name: api
...
```

```
$ etcdctl ls /vulcand/backends
/vulcand/backends/example.api.default
/vulcand/backends/example.web.default

$ etcdctl ls /vulcand/frontends
/vulcand/frontends/example.web.default
/vulcand/frontends/example.api.default

$ etcdctl get /vulcand/frontends/example.api.default/frontend
{"Type":"http","BackendId":"example.api.default","Route":"Host(`www.example.com`) && Path(`/api`)"}
```
