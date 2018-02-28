package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	avro "github.com/Guazi-inc/go-avro"
	"github.com/asaskevich/govalidator"
)

const (
	TEST_COMPATIBILITY = "/compatibility/subjects/%s-value/versions/latest"
	ENV_REGISTRY       = "SCHEMA_REGISTRY_ADDR"
	EXPECT_RETURN      = `{"is_compatible":true}`
)

type schemas []string

func (i *schemas) String() string {
	return fmt.Sprintf("%s", *i)
}
func (i *schemas) Set(value string) error {
	*i = append(*i, value)
	return nil
}

var schema schemas

func main() {
	parseflag()

	registryURL := os.Getenv(ENV_REGISTRY)
	if registryURL == "" {
		fmt.Println("have not set SCHEMA_REGISTRY_ADDR in the env,cannot check compatibility")
		os.Exit(1)
	}
	urlBase := registryURL + TEST_COMPATIBILITY

	schemas := make([]string, 0)
	for _, schema := range schema {
		contents, err := ioutil.ReadFile(schema)
		checkErr(err)
		schemas = append(schemas, string(contents))
	}

	for _, rawSchema := range schemas {
		parsedSchema, err := avro.ParseSchema(rawSchema)
		sch, ok := parsedSchema.(*avro.RecordSchema)
		if !ok {
			checkErr(errors.New("Not a Record schema"))
		}
		checkErr(err)

		subject := govalidator.CamelCaseToUnderscore(sch.Name)
		urlpath := fmt.Sprintf(urlBase, subject)
		val := map[string]string{"schema": sch.String()}
		temp, err := json.Marshal(val)
		checkErr(err)
		request, err := getRequest("POST", urlpath, bytes.NewReader(temp))
		checkErr(err)

		resp, err := http.DefaultClient.Do(request)
		checkErr(err)
		if resp.StatusCode != 200 {
			err := fmt.Errorf("failed to check compatibility.statuscode:%v", resp.StatusCode)
			checkErr(err)
		}
		body, err := ioutil.ReadAll(resp.Body)
		checkErr(err)
		if string(body) == EXPECT_RETURN {
			fmt.Println(subject+"-value", "ok")
		} else {
			checkErr(errors.New(subject + "-value " + string(body)))
		}
	}

}

func getRequest(method, urlpath string, body io.Reader) (*http.Request, error) {
	request, err := http.NewRequest(method, urlpath, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/vnd.schemaregistry.v1+json")
	if method == "POST" {
		request.Header.Set("Content-Type", "application/vnd.schemaregistry.v1+json")
	}
	return request, nil
}

func parseflag() {
	flag.Var(&schema, "schema", "path to avsc schema file")
	flag.Parse()
	if len(schema) == 0 {
		fmt.Println("At least one --schema flag is required")
		os.Exit(1)
	}
}

func checkErr(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
