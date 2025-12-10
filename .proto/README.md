# Protocol Buffers

This directory contains `.proto` files for Google Photos API communication.

## Generating Go Code

### Prerequisites

```bash
# Install protoc-gen-go
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
```

### Generate All

From the project root directory:

```bash
export PATH=$PATH:$(go env GOPATH)/bin

for proto in .proto/*.proto; do
  name=$(basename "$proto" .proto)
  protoc --proto_path=. --go_out=. --go_opt=M.proto/${name}.proto=github.com/viperadnan-git/gogpm/internal/pb .proto/${name}.proto
done
```

### Generate Single File

```bash
protoc --proto_path=. --go_out=. --go_opt=M.proto/MessageName.proto=github.com/viperadnan-git/gogpm/internal/pb .proto/MessageName.proto
```

## Creating New Proto Files

Use [blackboxprotobuf](https://github.com/nccgroup/blackboxprotobuf) to reverse-engineer proto definitions from encoded messages:

```python
# pip install bbpb
import blackboxprotobuf

protobuf_hex = "hex_encoded_message"
message_name = "MessageName"

protobuf_bytes = bytes.fromhex(protobuf_hex)
decoded_data, message_type = blackboxprotobuf.decode_message(protobuf_bytes)

blackboxprotobuf.export_protofile({message_name: message_type}, f".proto/{message_name}.proto")
```

After creating the proto file, add the go_package option:

```protobuf
syntax = "proto3";

option go_package = "github.com/viperadnan-git/gogpm/internal/pb";

message MessageName {
  // fields
}
```
