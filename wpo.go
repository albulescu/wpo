package main

import (
    "os"
    "runtime"
    "fmt"
    "flag"
)

type Module struct {
    module string
    action string
    fn string
}

func usage() {
    fmt.Print("usage: wpo [plugin|theme|options] [action] [params]\n")
    os.Exit(2)
}

func plugin( action string ) {

    var CommandLine = flag.NewFlagSet("params", flag.ExitOnError)

    instance := CommandLine.String("instance","2342562","The instance")

    CommandLine.Usage = usage;
    CommandLine.Parse(os.Args[3:])

    fmt.Printf("Instance: %s\n", *instance)
}

func plugin_install() {

}

func theme( action string ) {
    fmt.Print("Theme is not implemented")
    os.Exit(1)
}

func register_module(*modules []Module, name string, action string, fn func) {
    append(modules, Module{name:name, action:action, fn:""})
}

func main() {

    if runtime.GOOS != "linux" {
        fmt.Print("This application works only on linux\n")
        os.Exit(1)
    }

    if len(os.Args) < 2 {
        usage()
    }
    
    module := os.Args[1]
    action := os.Args[2]

    var modules []Module;

    append(modules, Module{name:"plugin", action:"install", fn:plugin_install})
}
