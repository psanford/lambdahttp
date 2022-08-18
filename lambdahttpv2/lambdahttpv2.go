package lambdahttpv2

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambdacontext"
)

type LambdaHandler func(context.Context, events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error)

func NewLambdaHandler(h http.Handler) LambdaHandler {
	return func(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
		w := newRespsonseWriter()
		r := newRequest(ctx, req)

		h.ServeHTTP(w, r)

		return w.Response(), nil
	}
}

func newRespsonseWriter() *ResponseWriter {
	header := make(http.Header)

	return &ResponseWriter{
		header:     header,
		statusCode: http.StatusOK,
	}
}

func newRequest(ctx context.Context, req events.APIGatewayV2HTTPRequest) *http.Request {
	u := url.URL{
		Host:     req.Headers["Host"],
		Scheme:   req.Headers["X-Forwarded-Proto"],
		Path:     req.RawPath,
		RawQuery: req.RawQueryString,
	}

	var bodyReader io.Reader = bytes.NewBufferString(req.Body)
	if req.IsBase64Encoded {
		bodyReader = base64.NewDecoder(base64.StdEncoding, bodyReader)
	}

	method := req.RequestContext.HTTP.Method

	ctx = withAPIGWv2ReqContext(ctx, req)
	httpReq, err := http.NewRequestWithContext(ctx, method, u.String(), bodyReader)
	if err != nil {
		panic(err)
	}

	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	lc, _ := lambdacontext.FromContext(ctx)
	httpReq.Header.Set("X-LambdaHttp-Aws-Request-Id", lc.AwsRequestID)
	httpReq.Header.Set("X-LambdaHttp-Function-Arn", lc.InvokedFunctionArn)

	for _, cookie := range req.Cookies {
		httpReq.Header.Add("cookie", cookie)
	}

	httpReq.RemoteAddr = req.RequestContext.HTTP.SourceIP
	return httpReq
}

type ResponseWriter struct {
	header     http.Header
	b          bytes.Buffer
	statusCode int
}

func (w *ResponseWriter) Header() http.Header {
	return w.header
}

func (w *ResponseWriter) Write(p []byte) (int, error) {
	return w.b.Write(p)
}

func (w *ResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

func (w *ResponseWriter) Response() events.APIGatewayProxyResponse {
	headers := make(map[string]string)
	for k, vals := range w.header {
		headers[k] = strings.Join(vals, ",")
	}

	return events.APIGatewayProxyResponse{
		StatusCode:      w.statusCode,
		Headers:         headers,
		IsBase64Encoded: true,
		Body:            base64.StdEncoding.EncodeToString(w.b.Bytes()),
		// MultiValueHeaders isn't respected for v2
	}
}

type ctxKey string

var (
	apiReqContextKey = ctxKey("apigwv2_req")
)

func APIGWv2ReqFromContext(ctx context.Context) events.APIGatewayV2HTTPRequest {
	reqI := ctx.Value(apiReqContextKey)
	if reqI == nil {
		return events.APIGatewayV2HTTPRequest{}
	}
	return reqI.(events.APIGatewayV2HTTPRequest)
}

func withAPIGWv2ReqContext(ctx context.Context, req events.APIGatewayV2HTTPRequest) context.Context {
	return context.WithValue(ctx, apiReqContextKey, req)
}
