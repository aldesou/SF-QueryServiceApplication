package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"queryservice/logger"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type queryHandler struct {
	logger logger.Logger
}

type queryRequest struct {
	Name string `json:"name" binding:"required"`
}

type queryResponse struct {
	Addresses []string `json:"addresses"`
}

func (qh *queryHandler) query(c *gin.Context) {
	correlationId := uuid.New().String()

	var qr queryRequest
	if err := c.BindJSON(&qr); err != nil {
		qh.logger.LogError(correlationId, fmt.Sprintf("Unable to decode request: %v", err))
		c.JSON(http.StatusBadRequest, gin.H{"error": "unable to decode request"})
		return
	}

	qh.logger.LogInfo(correlationId, fmt.Sprintf("Query name: %s", qr.Name))

	ips, err := net.DefaultResolver.LookupIPAddr(context.Background(), qr.Name)
	if err != nil {
		if strings.Contains(err.Error(), "no such host") {
			qh.logger.LogInfo(correlationId, "Name not found")
			c.JSON(http.StatusNotFound, gin.H{"error": "name not found"})
		} else {
			qh.logger.LogError(correlationId, fmt.Sprintf("Unable to resolve address: %v", err))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to resolve address"})
		}
		return
	}

	addrs := make([]string, 0)

	for _, ip := range ips {
		addrs = append(addrs, ip.IP.String())
	}

	resp := queryResponse{addrs}
	qh.logger.LogInfo(correlationId, fmt.Sprintf("Query response: %s", resp))

	c.JSON(http.StatusOK, resp)
}

func getEnvironmentVariable(key string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		panic(fmt.Sprintf("Environment variable \"%s\" not defined", key))
	}

	return value
}

func main() {
	var logFolderPath string
	flag.StringVar(&logFolderPath, "logFolderPath", "", "Path to folder where logs will be written")

	var host string
	flag.StringVar(&host, "host", "", "Server host used for binding")

	var port string
	flag.StringVar(&port, "port", "", "Server port used for binding")

	flag.Parse()

	if len(logFolderPath) == 0 {
		logFolderPath = getEnvironmentVariable("Fabric_Folder_App_Log")
	}

	logFilePath := filepath.Join(logFolderPath, "service.log")

	correlationId := uuid.New().String()

	logger := logger.NewFileLogger(logFilePath)
	defer logger.Close()

	if len(host) == 0 {
		host = getEnvironmentVariable("Fabric_Endpoint_IPOrFQDN_ServiceEndpoint")
	}

	if len(port) == 0 {
		port = getEnvironmentVariable("Fabric_Endpoint_ServiceEndpoint")
	}

	handler := queryHandler{logger}

	router := gin.Default()
	router.POST("/query", handler.query)

	addr := fmt.Sprintf("%s:%s", host, port)
	server := &http.Server{Addr: addr, Handler: router}

	go func() {
		logger.LogInfo(correlationId, fmt.Sprintf("Starting QueryService server on address %s", addr))

		if err := server.ListenAndServe(); err != nil {
			logger.LogInfo(correlationId, fmt.Sprintf("QueryService server exited with error \"%v\"", err))
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGINT)

	// Wait for ctrl+c
	<-c

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	logger.LogInfo(correlationId, "Stopping QueryService server")

	if err := server.Shutdown(ctx); err != nil {
		logger.LogInfo(correlationId, fmt.Sprintf("QueryService server shutdown error \"%v\"", err))
	}
}
