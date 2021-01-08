FROM golang:1.15.5-buster AS build

MAINTAINER Nikita Lobaev

RUN mkdir /go/src/Technopark_DB

COPY . /go/src/Technopark_DB

WORKDIR /go/src/Technopark_DB

RUN go get -d -v "github.com/labstack/echo/v4" "github.com/lib/pq"
RUN go build -o technopark_db main.go handlers.go

FROM ubuntu:20.04 AS release

MAINTAINER Nikita Lobaev

RUN apt-get -y update && apt-get install -y locales gnupg2
RUN locale-gen en_US.UTF-8
RUN update-locale LANG=en_US.UTF-8

ENV PGVER 12
ENV DEBIAN_FRONTEND noninteractive
RUN apt-get update -y && apt-get install -y postgresql postgresql-contrib

USER postgres

COPY db.sql /home

WORKDIR /home

RUN /etc/init.d/postgresql start &&\
    psql --command "CREATE USER forums_user WITH SUPERUSER PASSWORD 'forums_user';" &&\
    createdb -E UTF8 forums &&\
    psql --command "\i '/home/db.sql'" &&\
    /etc/init.d/postgresql stop

RUN echo "listen_addresses='*'\n" >> /etc/postgresql/$PGVER/main/postgresql.conf
RUN echo "host all  all    0.0.0.0/0  md5" >> /etc/postgresql/$PGVER/main/pg_hba.conf

VOLUME ["/etc/postgresql", "/var/log/postgresql", "/var/lib/postgresql"]

USER root

COPY --from=build /go/src/Technopark_DB/technopark_db /usr/bin/technopark_db

# EXPOSE 5432
# EXPOSE 5000

CMD service postgresql start && technopark_db
