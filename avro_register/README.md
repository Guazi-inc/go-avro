schema register Tool 
===============================

**Usage**:

`go run avro_register.go --schema foo.avsc --schema bar.avsc`

**Command line flags**:

`--schema` - absolute or relative path to Avro schema file. Multiple of those are allowed but at least one is required.

**Register schema**:
set registry address to environment variable: SCHEMA_REGISTRY_ADDR 
