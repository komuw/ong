# specify the base image to  be used for the application, alpine or ubuntu
FROM golang:1.19

# create a working directory inside the image
WORKDIR /app

RUN apt-get update -y;apt-get -y install libnss3-tools procps psmisc telnet iputils-ping nano curl wget

RUN mkdir -p /root/.pki/nssdb

# copy Go modules and dependencies to image
COPY go.mod /app

# download Go modules and dependencies
RUN go mod download

# copy directory files i.e all files ending with .go
COPY . /app

RUN pwd; ls -lsha

# compile application
RUN go build -race -o theApp example/main.go

# tells Docker that the container listens on specified network ports at runtime
EXPOSE 65080
EXPOSE 65081
EXPOSE 65060

# command to be used to execute when the image is used to start a container
CMD [ "./theApp" ]

# docker build -t onger .
# docker run -it --entrypoint /bin/bash onger:latest
# docker run -it onger:latest