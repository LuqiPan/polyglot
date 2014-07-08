package main

import (
  "github.com/gin-gonic/gin"
  "github.com/nu7hatch/gouuid"
  "github.com/streadway/amqp"
  "encoding/json"
  "log"
  "fmt"
  // "net/http"
  // "reflect"
)


func failOnError(err error, msg string) {
  if err != nil {
    log.Fatalf("%s: %s", msg, err)
    panic(fmt.Sprintf("%s: %s", msg, err))
  }
}

func input(c *gin.Context) {
  c.Req.ParseForm()
  c.Req.ParseMultipartForm(1024)
  c.Keys = make(map[string]interface{})  
  // marshal the HTTP request struct into JSON
  req_json, err := json.Marshal(c.Req)
  failOnError(err, "Failed to marshal the request")  
  c.Keys["request"] = string(req_json)
}

func process(c *gin.Context) {
  conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
  failOnError(err, "Failed to connect to RabbitMQ")
  defer conn.Close()

  ch, err := conn.Channel()
  failOnError(err, "Failed to open a channel")
  defer ch.Close()

  // declare the response queue used to receive responses from the responders
  replyq, err := ch.QueueDeclare(
    "RPC",   // name
    false,   // durable
    false,   // delete when usused
    false,   // exclusive
    false,   // noWait
    nil,     // arguments
  )
  
  // assert type of the body
  body := c.Keys["request"].(string)
  
  // publish the request into the polyglot queue
  corrId, _ := uuid.NewV4()
  err = ch.Publish(
    "",         // default exchange
    "polyglot", // routing key
    false,      // mandatory
    false,
    amqp.Publishing {
      DeliveryMode:  amqp.Persistent,
      ContentType:   "application/json",
      CorrelationId: corrId.String(),
      ReplyTo:       replyq.Name,
      Body:          []byte(body),
    })
  failOnError(err, "Failed to publish a message")  

  // wait to receive 
  msgs, err := ch.Consume(
    replyq.Name,     // queue
    "process",       // consumer
    true,            // auto acknowledge
    false,           // exclusive
    false,           // no local
    false,           // no wait
    nil,             // table
  )
  failOnError(err, "Failed to consume message")
  
  ret := make(chan []byte)
  go func() {
    for d := range msgs {
      ret <- d.Body
    }
  }()  
  response := string(<-ret)
    
  err = ch.Cancel("send", false)
  failOnError(err, "Failed to cancel channel") 
  
  c.Keys["response"] = string(response)
}


func output(c *gin.Context) {
  // get response JSON array 
  res := c.Keys["response"].(string)
  
  // unmarshal JSON into status, headers and body
  var r interface{}
  err := json.Unmarshal([]byte(res), &r); if err == nil {
    response := r.([]interface{})
    status := response[0]
    headers := response[1].(map[string]interface{})
    body := response[2]
    
    // write headers
    for k, v := range headers {
      c.Writer.Header().Set(k, v.(string))
    }
    s, _ := status.(float64)
    b, _ := body.(string)

    // write status and body to response
    c.Writer.WriteHeader(int(s))
    c.Writer.Write([]byte(b))
  }
}