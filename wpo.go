package main

import (
    "os"
    "runtime"
    "fmt"
    "flag"
    "log"
    "regexp"
    "net"
    "bufio"
    "bytes"
    "strings"
    "path/filepath"
    "io/ioutil"
    "database/sql"
    _ "github.com/go-sql-driver/mysql"
)

type Module struct {
    name string
    action string
    fn func()
}

type Setting struct {
    name string
    value string
}

var modules []Module
var settings []Setting

func usage() {
    fmt.Print("usage: wpo [plugin|theme|options] [action] [params]\n")
    os.Exit(2)
}

func error(message string) {
    fmt.Print(message)
    fmt.Print("\n")
    os.Exit(1)
}

func dbconnect() {
    
    if ! has_settings("DB_HOST", "DB_USER","DB_PASSWORD", "DB_NAME") {
        error("Database settings missing from wp-config.php file")
    }
    
    var connectURL bytes.Buffer

    connectURL.WriteString( get_setting("DB_USER") )
    connectURL.WriteString( ":" )
    connectURL.WriteString( get_setting("DB_PASSWORD") )
    connectURL.WriteString( "@tcp(" )
    connectURL.WriteString( getip(get_setting("DB_HOST")) )
    connectURL.WriteString( ":3306)/" )
    connectURL.WriteString( get_setting("DB_NAME") )

    db, err := sql.Open("mysql", connectURL.String())

    if err != nil {
        panic(err.Error())
    }

    defer db.Close()

    err = db.Ping()

    if err != nil {
        panic(err.Error())
    }

    // Prepare statement for reading data
    rows, err := db.Query("SELECT ID,user_login,user_email FROM wp_users")
    
    var id,user,email []byte

    fmt.Println("Take users from database:");

    // Fetch rows
    for rows.Next() {
        // get RawBytes from data
        err = rows.Scan(&id,&user,&email)

        if err != nil {
            panic(err.Error()) // proper error handling instead of panic in your app
        }

        fmt.Printf("ID:%s USER:%s EMAIL:%s\n", string(id), string(user), string(email))
    }
}

func wpdir( args ...string ) (string) {
    
    var path bytes.Buffer

    dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
    
    if err != nil {
        log.Fatal(err)
    }

    path.WriteString(dir)

    if len(args) > 0 {
        path.WriteString(args[0]);
    }

    return path.String()
}

func is_wordpress() (bool) {
    
    files, error := ioutil.ReadDir(wpdir())

    if error != nil {
        log.Fatal(error)
    }

    for i := 0; i < len(files); i++ {

        if !files[i].IsDir() && 
            strings.EqualFold( files[i].Name(), "wp-config.php" ) {
            return true
        }
    }

    return false
}

func plugin_install() {

    var args = flag.NewFlagSet("params", flag.ExitOnError)

    name := args.String("name", "","Plugin name")

    args.Parse(os.Args[3:])

    if len(*name) == 0 {
        error("Please specify plugin name")
    }

    dbconnect()
}

func getip( domain string ) (string) {

    addrs, err := net.LookupHost(domain)
    
    if err != nil {
        fmt.Printf("Oops: %v\n", err)
    }

    if len(addrs) > 0 {
        return addrs[0]
    }

    return "127.0.0.1"
}

func has_settings( args ...string) (bool) {
    
    found:=0
    
    for i:=0; i<len(args); i++ {
        
        for j:=0; j<len(settings); j++ {
            
            if strings.EqualFold(settings[j].name, args[i]) {
               found++
            }

            if found == len(args) {
                return true
            }
        }
    }

    return false
}

func get_setting(name string) (string) {

    for i:=0; i<len(settings); i++ {
        if strings.EqualFold(settings[i].name, name) {
            return settings[i].value
        }
    }

    return ""
}

func read_settings() {

    inFile, _ := os.Open(wpdir("/wp-config.php"))
    
    defer inFile.Close()
    
    scanner := bufio.NewScanner(inFile)
    
    scanner.Split(bufio.ScanLines) 

    re := regexp.MustCompile("define\\('(.*)',\\s*'(.*)'\\)")

    for scanner.Scan() {
        
        line:=strings.Trim(scanner.Text(),"\t ")
        
        matched, _ := regexp.MatchString("define\\('", line)

        if matched {
            
            matches := re.FindAllStringSubmatch(line, -1)

            if len(matches) > 0 {
                setting := Setting{name: matches[0][1], value: matches[0][2]}
                settings = append(settings, setting)
            }
        }
    }
}

func command(name string, action string, fn func()) {
    modules = append(modules, Module{name:name, action:action, fn:fn});
}

func execute(name string, action string) {
    
    for i := 0; i < len(modules); i++ {
        module := modules[i]
        if module.name == name && module.action == action {
            module.fn()
            return
        }
    }

    usage()
}

func main() {

    if runtime.GOOS != "linux" {
        fmt.Print("This application works only on linux\n")
        os.Exit(1)
    }

    if ! is_wordpress() {
        error("Current path is not a wordpress directory")
    }

    if len(os.Args) < 3 { usage() }
    
    read_settings()

    command("plugin","install", plugin_install);
    
    execute(os.Args[1], os.Args[2]);
}
