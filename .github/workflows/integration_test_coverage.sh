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


rate_limit_test(){
  VEGETA=$(which vegeta)

  rm -rf vegeta_results.text vegeta_results.json

  echo "GET https://localhost:65081/check/67" | \
    $VEGETA attack -duration=20s -rate=90/s -max-workers=500 | \
    tee vegeta_results.text | \
    $VEGETA report --type json >> vegeta_results.json

  echo ''
  cat vegeta_results.json
  echo ''

  number_of_success=$(cat vegeta_results.json | jq '.status_codes."200"')
  if [[ $"$number_of_success" -gt 1780 ]]; then
    # Actually, we would expect 1800 successes(20 *90) since the sending rate is 90/secs
    # which is below the ratelimit of 100/sec.
    # But ratelimiting is imprecise; https://github.com/komuw/ong/issues/235
    echo "expected at least 1780 successful requests"
    exit 61;
  fi

  errors=$(cat vegeta_results.json | jq '.errors[0]')
  string='My long string'
  if [[ $errors == *"429 Too Many Requests"* ]]; then
    echo "" # it's there 
  else
    echo "expected ratelimiting errors"
    exit 61;
  fi
}
rate_limit_test
