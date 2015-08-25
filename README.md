# romulusd

[![GoDoc](https://godoc.org/github.com/timelinelabs/romulus?status.svg)](https://godoc.org/github.com/timelinelabs/romulus)

Automagically register your kubernetes services in vulcan proxy!

## Usage

```
$ romulusd --help
usage: romulusd [<flags>]

A utility for automatically registering Kubernetes services in Vulcand

Flags:
  --help           Show help (also see --help-long and --help-man).
  --vulcan-key="vulcand"
                   default vulcand etcd key
  -e, --etcd=http://127.0.0.1:2379
                   etcd peers
  -t, --etcd-timeout=5s
                   etcd request timeout
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
  -d, --debug      Enable debug logging. e.g. --log-level debug
  -l, --log-level=info
                   log level. One of: fatal, error, warn, info, debug
  --debug-etcd     Enable cURL debug logging for etcd
```

Set up your kubernetes service with a label and some options annotations:

*NOTE*: all labels and annotations are under the prefix `romulus/`
*NOTE 2*: set the etcd vulcand prefix with the label `romulus/vulcanKey`. If not set, then the default key is used (see flag `--vulcan-key`)

```yaml
apiVersion: v1
kind: Service
metadata:
  name: example
  annotations:
    romulus/host: 'www.example.com'
    romulus/pathRegexp: '/guestbook/.*'
    romulus/frontendSettings: '{"FailoverPredicate":"(IsNetworkError() || ResponseCode() == 503) && Attempts() <= 2"}}'
    romulus/backendSettings: '{"KeepAlive": {"MaxIdleConnsPerHost": 128, "Period": "4s"}}'
  labels:
    name: example
    romulus/vulcanKey: 'vulcand-test'
    romulus/type: external # <-- Will ensure SVC-SELECTORs specified (e.g. 'type=external') are present in Labels.
spec: 
...
```

When you create the service, romulusd will create keys in etcd for vulcan!

*NOTE*: IDs for backends and frontends are constructed as follows: `[<port name>.]<kube resource name>.<namespace>`

```
$ kubectl.sh get svc,endpoints -l romulus/type=external
NAME           LABELS                            SELECTOR            IP(S)           PORT(S)
frontend       name=frontend,type=external       name=frontend       10.247.242.50   80/TCP
NAME           ENDPOINTS
frontend       10.246.1.7:80,10.246.1.8:80,10.246.1.9:80

$ etcdctl get /vulcand-test/backends/example.default/backend
{"Id":"example.default","Type":"http","Settings":{"KeepAlive":{"MaxIdleConnsPerHost":128,"Period": "4s"}}}

$ etcdctl get /vulcand-test/frontends/example.default/frontend
{"Id": "example.default","Type":"http","BackendId":"example.default","Route":"Host(`www.example.com`) && PathRegexp(`/guestbook/.*`)","Settings":{"FailoverPredicate":"(IsNetworkError() || ResponseCode() == 503) && Attempts() <= 2"}}

$ etcdctl ls /vulcand-test/backends/example.default/servers
/vulcand-test/backends/example.default/servers/10.246.1.8
/vulcand-test/backends/example.default/servers/10.246.1.9
/vulcand-test/backends/example.default/servers/10.246.1.7
```

## Multi Port Services

If your service has multiple ports, romulusd will create a frontend for each.

Separate options by appending the port name as a suffix (e.g. `romulus/path.api`). If no matching `romulus/<opt>.<port_name>` option exists, then the `romulus/<opt>` option will be used.

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
/vulcand/backends/api.example.default
/vulcand/backends/web.example.default

$ etcdctl ls /vulcand/frontends
/vulcand/frontends/web.example.default
/vulcand/frontends/api.example.default

$ etcdctl get /vulcand/frontends/api.example.default/frontend
{"Id":"api.example.default","Type":"http","BackendId":"api.example.default","Route":"Host(`www.example.com`) && Path(`/api`)"}
```
