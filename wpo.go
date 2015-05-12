package main

import (
    "flag"
    "os"
    "runtime"
    "fmt"
)

func usage() {
    fmt.Fprintf(os.Stderr, "usage: myprog [inputfile]\n")
    os.Exit(2)
}

func main() {

    if runtime.GOOS != "linux" {
        fmt.Print("This application works only on linux\n")
        os.Exit(1)
    }

    var CommandLine = flag.NewFlagSet(os.Args[1], flag.ExitOnError)

    action := CommandLine.String("i","foo","A string")

    flag.Usage = usage
    flag.Parse()

    fmt.Print(*action)
    fmt.Print(CommandLine)
}
