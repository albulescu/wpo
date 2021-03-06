package main

import (
    "os"
    "runtime"
    "fmt"
    "flag"
    "log"
    "regexp"
    "net"
    "io"
    "net/http"
    "bufio"
    "archive/zip"
    "bytes"
    "strings"
    "path/filepath"
    "io/ioutil"
    "database/sql"
    _ "github.com/go-sql-driver/mysql"
    "github.com/jeffail/gabs"
)

const (

    WORDPRESS_PLUGIN_API string = "https://api.wordpress.org/plugins/info/1.0/{SLUG}.json"
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
        error(err.Error())
    }

    defer db.Close()

    err = db.Ping()

    if err != nil {
        error(err.Error())
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
            error(err.Error()) // proper error handling instead of error in your app
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

func build_url( url string, vars map[string]string) (string) {

    for key, value := range vars {
        
        var placeholder bytes.Buffer

        placeholder.WriteString("{")
        placeholder.WriteString(key)
        placeholder.WriteString("}")

        url = strings.Replace(url, placeholder.String(), value, -1);
    }

    return url
}

func Unzip(src, dest string) (error interface{}) {
    r, err := zip.OpenReader(src)
    if err != nil {
        return err
    }
    defer r.Close()

    for _, f := range r.File {
        rc, err := f.Open()
        if err != nil {
            return err
        }
        defer rc.Close()

        path := filepath.Join(dest, f.Name)
        if f.FileInfo().IsDir() {
            os.MkdirAll(path, f.Mode())
        } else {
            f, err := os.OpenFile(
                path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
            if err != nil {
                return err
            }
            defer f.Close()

            _, err = io.Copy(f, rc)
            if err != nil {
                return err
            }
        }
    }

    return nil
}

func download_plugin(slug string) {

    fmt.Println("Reading api...");

    resp, err := http.Get(build_url(WORDPRESS_PLUGIN_API, map[string]string{"SLUG":slug}))

    if err != nil {
        error("Fail to read plugins api")
    }

    if resp.StatusCode == 404 {
        error("Plugin not found");
    }

    defer resp.Body.Close()

    body, err := ioutil.ReadAll(resp.Body)

    if err != nil {
        error("Fail to read plugins api body")
    }

    if strings.EqualFold(string(body), "null") {
        error("Plugin not found")
    }

    parsed, err := gabs.ParseJSON(body)

    if err != nil {
        error("Fail to read json")
    }

    plugin_zip, ok := parsed.Path("download_link").Data().(string)
    plugin_name, ok := parsed.Path("name").Data().(string)

    if !ok {
        error("Invalid data received from api")
    }

    fmt.Printf("Downloading \"%s\" from %s...\n", plugin_name, plugin_zip)

    zip_resp, get_zip_err := http.Get(plugin_zip)
    defer resp.Body.Close()

    if get_zip_err != nil {
        error("Fail to download zip")
    }

    temp_file, temp_file_err := ioutil.TempFile(os.TempDir(), "wpo_plugin")

    if temp_file_err != nil {
        error("Fail to create temporary file")
    }

    defer temp_file.Close();

    zip_data, read_zip_error := ioutil.ReadAll(zip_resp.Body)

    if read_zip_error != nil {
        error("Fail to read bytes from response")
    }

    temp_file.Write(zip_data);

    zip, zip_open_err := zip.OpenReader(temp_file.Name())

    if zip_open_err != nil {
        error("Not a zip file")
    }

    defer zip.Close()

    fmt.Println("Extracting zip...");

    unzip_error := Unzip(temp_file.Name(), wpdir("/wp-content/plugins"))

    if unzip_error != nil {
        error("Not a zip file")
    }

    fmt.Println("Remove temporary file");

    os.Remove(temp_file.Name())
}

func is_wordpress() (bool) {
    return true
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

func dirExists(path string) (bool) {
    _, err := os.Stat(path)
    if err == nil { return true }
    if os.IsNotExist(err) { return false }
    return false
}

func plugin_install() {

    var args = flag.NewFlagSet("params", flag.ExitOnError)

    slug := args.String("slug", "","Plugin slug")

    args.Parse(os.Args[3:])

    if len(*slug) == 0 {
        error("Please specify plugin slug")
    }

    var path bytes.Buffer

    path.WriteString("/wp-content/plugins/")
    path.WriteString(*slug)

    exist := dirExists(wpdir(path.String()))

    if exist {
        error("Plugin already exist")
    }

    download_plugin(*slug)

    fmt.Printf("Installed at: %s", path.String())

    os.Exit(0)
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
