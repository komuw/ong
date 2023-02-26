#!/usr/bin/env bash
shopt -s nullglob globstar
set -x # have bash print command been ran
set -e # fail if any command fails

# This file tests that the examples in the /example/ directory will continue to work correctly.

https_redirection(){
  status_code=$(curl -k --write-out '%{http_code}' --silent --output /dev/null "http://127.0.0.1:65080/health")
  if [[ "$status_code" -ne 308 ]] ; then
    echo "expected 127.0.0.1 to be redirected to localhost"
    exit 61;
  fi

  status_code=$(curl -k --write-out '%{http_code}' --silent --output /dev/null "https://localhost:65081/health")
  if [[ "$status_code" -ne 200 ]] ; then
    echo "expected success"
    exit 61;
  fi
}
https_redirection

pprof(){
  status_code=$(curl -k --write-out '%{http_code}' --silent --output /dev/null "http://127.0.0.1:65060/debug/pprof/profile?seconds=3")
  if [[ "$status_code" -ne 200 ]] ; then
    echo "expected success"
    exit 61;
  fi
}
pprof

static_file_server(){
  status_code=$(curl -k --write-out '%{http_code}' --silent --output /dev/null "https://localhost:65081/staticAssets/hello.css")
  if [[ "$status_code" -ne 401 ]] ; then
    echo "expected basic auth failure"
    exit 61;
  fi 

  status_code=$(curl -u user:some-long-passwd -k --write-out '%{http_code}' --silent --output /dev/null "https://localhost:65081/staticAssets/hello.css")
  if [[ "$status_code" -ne 200 ]] ; then
    echo "expected success"
    exit 61;
  fi 
}
static_file_server

check_age(){
  status_code=$(curl -k --write-out '%{http_code}' --silent --output /dev/null "https://localhost:65081/check/67")
  if [[ "$status_code" -ne 200 ]] ; then
    echo "expected success"
    exit 61;
  fi 
}
check_age

login(){
  status_code=$(curl -k --write-out '%{http_code}' --silent --output /dev/null "https://localhost:65081/login")
  if [[ "$status_code" -ne 200 ]] ; then
    echo "expected success"
    exit 61;
  fi 

  # with slash suffix
  status_code=$(curl -k --write-out '%{http_code}' --silent --output /dev/null "https://localhost:65081/login/")
  if [[ "$status_code" -ne 200 ]] ; then
    echo "expected success"
    exit 61;
  fi
}
login

panic(){
  status_code=$(curl -k --write-out '%{http_code}' --silent --output /dev/null "https://localhost:65081/panic")
  if [[ "$status_code" -ne 500 ]] ; then
    echo "expected http 500"
    exit 61;
  fi 
}
panic

