package main

import (
  "bytes"
  "fmt"
  "go/format"
  "io/fs"
  "io/ioutil"
  "log"
  "os"
  "path/filepath"
  "strings"
  "text/template"
  "flag"
)

var args = os.Args

var blobFileName *string
var staticFolder *string
var packageName *string

func init() {
  blobFileName = flag.String("output", "", "output filename for generation")
  staticFolder = flag.String("input", "", "input files to embed")
  packageName = flag.String("packagename", "", "package name for the output file, usually the same as the package name where the embed files will be called from")
  fmt.Println(blobFileName, packageName, staticFolder)
  flag.Parse()

  if _, err := os.Stat(*blobFileName); os.IsNotExist(err) {
    pathComponents := strings.Split(*blobFileName, "/")
    finalPath := pathComponents[0:len(pathComponents) - 1]

    directory := strings.Join(finalPath, "/")
    os.Mkdir(directory, os.ModePerm)
  }
}

type templateConfig struct {
  Files map[string][]byte
  PackageName string
}

var funcMap = map[string]interface{}{"conv": fmtSlice}
var templateBuilder = template.Must(template.New("").Funcs(funcMap).Parse(`package {{ .PackageName }}

// Code generated by go generate, DO NOT EDIT


func init() {
  {{- range $name, $file := .Files}}
    vault.Add("{{ $name }}", []byte{ {{ conv $file }} })
  {{ end }}
}

type Vault struct {
  storageUnit map[string][]byte
}

func newVault() *Vault {
  return &Vault{storageUnit: make(map[string][]byte)}
}

func (vault *Vault) Add(filename string, content []byte) {
  vault.storageUnit[filename] = content
}

func (vault *Vault) GetFile(filename string) []byte {
  return vault.storageUnit[filename]
}

var vault = newVault()

func Add(filename string, content []byte) {
  vault.Add(filename, content)
}

func Get(filename string) []byte {
  return vault.GetFile(filename)
}
`))

func fmtSlice(s []byte) string {
  builder := strings.Builder{}

  for _, b := range s {
    builder.WriteString(fmt.Sprintf("%d,", int(b)))
  }

  return builder.String()
}

func main() {
  if _, err := os.Stat(*staticFolder); os.IsNotExist(err) {
    log.Fatal(fmt.Sprintf("The folder, %v, does not exist", staticFolder))
  }

  files := make(map[string][]byte)

  filepath.Walk(*staticFolder, func(path string, info fs.FileInfo, err error) error {
    relativeFilePath := filepath.ToSlash(strings.TrimPrefix(path, *staticFolder))

    if info.IsDir() {
      fmt.Printf("Skipping directory, %v", path)
    } else {
      if bytes, err := ioutil.ReadFile(path); err != nil {
        log.Fatal(fmt.Sprintf("There was a problem reading %v: %v", path, err))
      } else {
        files[relativeFilePath] = bytes
      }
    }

    return err
  })

  blobFile, err := os.Create(*blobFileName)

  defer blobFile.Close()
  if err != nil {
    log.Fatal(fmt.Sprintf("There was a problem creating %v", blobFileName))
  }

  builder := &bytes.Buffer{}
  config := &templateConfig{Files: files, PackageName: *packageName}
  if err := templateBuilder.Execute(builder, config); err != nil {
    log.Fatal(fmt.Sprintf("There was a problem generating the embeds: %v", err))
  }

  if data, err := format.Source(builder.Bytes()); err != nil {
    log.Fatal(fmt.Sprintf("There was an error formating the generated file, %v", err))
  } else {
    if err := ioutil.WriteFile(*blobFileName, data, os.ModePerm); err != nil {
      log.Fatal(fmt.Sprintf("There was a problem writing the blob file: %v", err))
    }
  }
}
