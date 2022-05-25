## About
`rsvz_checker` is an application which can help you receive information about the russian phone number by HTTP.
It can be helpful if you're using Asterisk or some other softswitch.   

## How it works
Compile application with the golang compiler, deploy to your server, setup config file (env variables) and that's all.

## Usage
```bash
git clone https://github.com/gostkov/rsvz_checker.git
cd rsvz_checker
go build
./rsvz_checker
(wait for message "Parsing completed" in the log.)
```

and now you can test this.

```bash
curl "http://127.0.0.1:8081/check/?num=73512470001"
{"code":351,"full_num":"73512470000","operator":"ПАО МегаФон","region":"г. Челябинск|Челябинская обл."}
```

You can return specific field if add `field=`
For example:
```bash
curl "http://127.0.0.1:8081/check/?num=73512470001&field=operator"
ПАО МегаФон
```

## Configuration
By default, application searing config file `rsvz_checker.env` in the current directory.
If you want to change destination of configuration file, just use argument `--config`

`SERVER_IP`, `SERVER_PORT` it means which ip address and port will listen application. 

`REFRESH_INTERVAL` this option set time interval after that rsvz_cheker will download new files from offical site.

`URLS` contain URL of files for downloading and parsing.

You can set this variable in the environment.
`export SERVER_PORT=8090`

Environment variables have the highest priority for usage.

