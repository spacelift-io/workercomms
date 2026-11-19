# workercomms

Go library for communication between Spacelift workers and Spacelift. The
Spacelift launcher and kube-workerpool-controller both import it.

```
go get github.com/spacelift-io/workercomms
```

## Packages

- messages defines the messages Spacelift and workers exchange and routes each
  received message to the code that handles its type.
- mqtt connects a worker pool to Spacelift over MQTT.
- pollclient receives messages for each worker by polling Spacelift over HTTP.
- pollcomms defines what the poll client and Spacelift send each other and how
  a poll request proves which worker pool it comes from.
- crypto encrypts and decrypts the work Spacelift assigns and generates the
  key pair that work is encrypted for.
- workload describes what a worker needs to know about an assigned run, intent
  task or scan before it starts.
- capability names the features that a launcher or a kube-workerpool-controller
  build supports, which each one reports when it registers a worker.
- workerapi defines what a workload sends Spacelift to report run state
  changes, intent results and scan events, and where it sends them.

## License

MIT, see LICENSE.
