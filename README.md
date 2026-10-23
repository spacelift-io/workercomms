# workercomms

Go library for communication between Spacelift workers and Spacelift. The
Spacelift launcher and kube-workerpool-controller both import it.

```
go get github.com/spacelift-io/workercomms
```

## Packages

- messages has the message envelope, its types, the MessageHandler that
  dispatches an envelope by type, and the Logger and PayloadHandler interfaces
  the transports take.
- mqtt is the MQTT client a worker pool connects with, plus the messages a
  worker sends over it.
- pollclient is the worker side of the HTTP long-poll transport. One poll loop
  runs per worker and signs a pool-key JWT for every request.
- pollcomms holds the long-poll wire types and the JWT signing and
  verification.
- crypto encrypts and decrypts assignment payloads and generates the worker
  key pair.
- workload holds the header fields every assignment payload starts with, the
  workload kinds and the environment variable names.
- workerapi holds the worker API paths and the bodies a workload sends to
  report run state changes, intent results and scan events.

## Versioning

The module stays on v0, so a breaking change ships as a minor version.

## License

MIT, see LICENSE.
