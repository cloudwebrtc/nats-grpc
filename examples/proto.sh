#!/bin/sh

# note: need protoc-gen-go-grpc@v1.1.0
# go get google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.1.0

OUT_DIR=./protos/echo
SRC_DIR=./protos


mkdir -p ${OUT_DIR}

protoc -I ${SRC_DIR} \
	--go_out=${OUT_DIR} \
	--go-grpc_out=${OUT_DIR} \
	${SRC_DIR}/echo.proto
