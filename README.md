# http-proxy-amqp

## Introduction
This is a amqp connection pool implementation
![Alt text](https://github.com/iyidan/http-proxy-amqp/raw/master/intro.jpg)

## Install
`go get github.com/iyidan/http-proxy-amqp`

## Run
`./http-proxy-amqp -config=path_to_config_file.json`

## Usage
`curl -XPOST 'http://127.0.0.1:35673/confirm_send?exchange={xx}&routingKey={xx}' -d 'msg'`

## Options
Please read the <a href="https://raw.githubusercontent.com/iyidan/http-proxy-amqp/master/config/config_default.json">config</a> file for more information.