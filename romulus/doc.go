// Package romulus contains all the logic for monitoring the kubernetes api
// and registering services with vulcan proxy
//
//    c, e := romulus.NewRegistrar(&romulus.Config{
//      PeerList:            eps,
//      APIVersion:          *kv,
//      KubeConfig:          kcc,
//      Selector:            *sl,
//      VulcanEtcdNAMESPACE: *vk,
//    })
//    if e != nil {
//      // handle configuration error
//    }
//
//    if e := romulus.Start(c); e != nil {
//      // handle runtime error
//    }
//    // NOTE: romulus DOES NOT clean up etcd on shutdown
//
// vulcan proxy documentation: https://docs.vulcand.io/index.html
//
// kubernetes documentation: https://github.com/GoogleCloudPlatform/kubernetes/tree/master/docs
package romulus

var (
	version = "v0.1.2"

	// SHA is the build sha
	SHA string
)
