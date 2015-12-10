package vulcand

var DefaultMiddleware = map[string]string{
	"redirect_ssl": `{
      "Priority": 1,
      "Type": "rewrite",
      "Middleware": {
        "Regexp": "^http://(.*)",
        "Replacement": "https://$1",
        "Rewritebody": false,
        "Redirect": true
      }
    }`,
	"trace": `{
      "Priority": 1,
      "Type": "trace",
      "Middleware": {
        "ReqHeaders": %s,
        "RespHeaders": %s,
        "Addr": "syslog://127.0.0.1:514",
        "Prefix": "@app"
      }
    }`,
	"auth": `{
      "Priority": 1,
      "Type": "auth",
      "Middleware": {
        "User": "%s",
        "Pass": "%s"
      }
    }`,
	"maintenance": `{
      "Priority": 1,
      "Type": "cbreaker",
      "Middleware": {
        "Condition": "ResponseCodeRatio(500, 600, 0, 600) > 0.9",
        "Fallback": {
          "Type": "response",
          "Action": {
            "StatusCode": 503,
            "Body": %q
          }
        },
        "FallbackDuration": 1000000000,
        "RecoveryDuration": 1000000000,
        "CheckPeriod": 100000000
      }
    }`,
}
