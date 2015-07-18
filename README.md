# romulusd

Automagically register your kubernetes services in vulcan proxy!

## Usage

```
$ romulusd --help
usage: romulusd [<flags>]

Flags:
  --help           Show help (also see --help-long and --help-man).
  -e, --etcd=http://127.0.0.1:2379
                   etcd peers
  -k, --kube=http://127.0.0.1:8080
                   kubernetes endpoint
  -U, --kube-user=KUBE-USER
                   kubernetes username
  -P, --kube-pass=KUBE-PASS
                   kubernetes password
  --kube-api="v1"  kubernetes api version
  -C, --kubecfg=KUBECFG
                   path to kubernetes cfg file
  -s, --svc-selector=type=external
                   service selectors. Leave blank for Everything(). Form: key=value
  --version        Show application version.
```

Set up your kubernetes service with a label and some options annotations:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: frontend
  annotations:
    host: 'www.example.com'
    path: 'guestbook'
    frontendSettings: '{"FailoverPredicate":"(IsNetworkError() || ResponseCode() == 503) && Attempts() <= 2"}}'
    backendSettings: '{"KeepAlive": {"MaxIdleConnsPerHost": 128, "Period": "4s"}}'
  labels:
    name: frontend
    type: external # <-- By default, services without this label will not be registered
spec: 
...
```

When you create the service, romulusd will create keys in etcd for vulcan!

```
$ kubectl.sh get svc,endpoints -l type=external
NAME           LABELS                            SELECTOR            IP(S)           PORT(S)
frontend       name=frontend,type=external       name=frontend       10.247.242.50   80/TCP
NAME           ENDPOINTS
frontend       10.246.1.7:80,10.246.1.8:80,10.246.1.9:80

$ kubectl get endpoints/frontend --template='{{ printf "%s\n" .metadata.uid }}'
52de7ac8-2ce8-11e5-8a86-0800279dd272

$ etcdctl get /vulcan/backends/52de7ac8-2ce8-11e5-8a86-0800279dd272/backend
{"Type":"http","Settings":{"KeepAlive":{"MaxIdleConnsPerHost":128}}}

$ kubectl get svc/frontend --template='{{ printf "%s\n" .metadata.uid }}'
52d87539-2ce8-11e5-8a86-0800279dd272

$ etcdctl get /vulcan/frontends/81aaba2e-2ce5-11e5-8a86-0800279dd272/frontend
{"Type":"http","BackendId":"52de7ac8-2ce8-11e5-8a86-0800279dd272","Route":"Host('www.example.com') && Path('guestbook')","Settings":{"FailoverPredicate":"(IsNetworkError() || ResponseCode() == 503) && Attempts() <= 2"}}

$ etcdctl ls /vulcan/backends/52de7ac8-2ce8-11e5-8a86-0800279dd272/servers
/vulcan/backends/52de7ac8-2ce8-11e5-8a86-0800279dd272/servers/10.246.1.8
/vulcan/backends/52de7ac8-2ce8-11e5-8a86-0800279dd272/servers/10.246.1.9
/vulcan/backends/52de7ac8-2ce8-11e5-8a86-0800279dd272/servers/10.246.1.7
```


