# protoc-gen-proxy

protoc plugin to extend grpc generation with proxy(multiplexing) functionality. This plugin is meant to be chained with the grpc plugin to add additional functionality to gRPC.

## Usage

```bash
protoc -I./tests/proto --plugin=proxy --proxy_out=plugins=grpc+proxy:tests/proto tests/proto/api.proto
```

The following will extend the generated gRPC protobuf definition to include a `grpc.UnaryInterceptor` that will route incoming requests to any additional hosts specified in the `metadata["targets"]` field.
