/* Licensed to the Apache Software Foundation (ASF) under one or more
contributor license agreements.  See the NOTICE file distributed with
this work for additional information regarding copyright ownership.
The ASF licenses this file to You under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with
the License.  You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License. */

package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	avro "github.com/Guazi-inc/go-avro"
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
	parseAndValidateArgs()

	for _, schema := range schema {
		contents, err := ioutil.ReadFile(schema)
		checkErr(err)
		reGen, err := avscRegen(string(contents))
		checkErr(err)
		file, err := os.Create(schema)
		checkErr(err)
		_, err = file.WriteString(reGen)
		if err != nil {
			file.Close()
			fmt.Println(err)
			os.Exit(1)
		}
		file.Close()
	}
}

func parseAndValidateArgs() {
	flag.Var(&schema, "schema", "Path to avsc schema file.")
	flag.Parse()

	if len(schema) == 0 {
		fmt.Println("At least one --schema flag is required.")
		os.Exit(1)
	}
}

func checkErr(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func avscRegen(content string) (string, error) {
	parsedSchema, err := avro.ParseSchema(content)
	if err != nil {
		return "", err
	}
	schema, ok := parsedSchema.(*avro.RecordSchema)
	if !ok {
		return "", errors.New("Not a Record schema")
	}
	if err != nil {
		return "", err
	}
	return schema.String(), nil
}
