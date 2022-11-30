FROM golang

COPY ./app /app
WORKDIR /app
RUN go mod tidy
RUN go build main.go
CMD /bin/bash -c /app/main