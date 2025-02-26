package glance

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

var (
	errNoContent      = errors.New("failed to retrieve any content")
	errPartialContent = errors.New("failed to retrieve some of the content")
)

const defaultClientTimeout = 5 * time.Second

var (
	insecureClientTransport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	defaultTransport = &http.Transport{}

	defaultClient = &http.Client{
		Timeout:   defaultClientTimeout,
		Transport: defaultTransport,
	}

	defaultInsecureClient = &http.Client{
		Timeout:   defaultClientTimeout,
		Transport: insecureClientTransport,
	}

	clientCache = sync.Map{}
)

type RequestDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// GetClient 返回一个 http.Client 指针，根据提供的代理 URL 和安全设置创建
func GetClient(proxyURL string, insecure bool) (*http.Client, error) {

	if client, ok := clientCache.Load(proxyURL); ok {
		return client.(*http.Client), nil
	}

	proxyURLParsed, err := url.Parse(proxyURL)
	if err != nil {
		return nil, fmt.Errorf("invalid proxy URL: %w", err)
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURLParsed),
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: insecure,
		},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
		IdleConnTimeout:     90 * time.Second,
	}

	client := &http.Client{
		Timeout:   defaultClientTimeout,
		Transport: transport,
	}

	clientCache.Store(proxyURL, client)
	return client, nil
}

func SetProxy(proxyURL string) error {
	proxyURLParsed, err := url.Parse(proxyURL)
	if err != nil {
		return fmt.Errorf("invalid proxy URL: %w", err)
	}

	setupTransport := func(transport *http.Transport, insecureSkipVerify bool) {
		transport.Proxy = http.ProxyURL(proxyURLParsed)
		if insecureSkipVerify {
			if transport.TLSClientConfig == nil {
				transport.TLSClientConfig = &tls.Config{}
			}
			transport.TLSClientConfig.InsecureSkipVerify = true
		}
	}

	setupTransport(defaultTransport, false)
	setupTransport(insecureClientTransport, true)

	_, err = GetClient(proxyURL, false)
	if err != nil {
		return err
	}
	_, err = GetClient(proxyURL, true)
	if err != nil {
		return err
	}

	return nil
}

func addBrowserUserAgentHeader(request *http.Request) {
	request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:123.0) Gecko/20100101 Firefox/123.0")
}

func decodeJsonFromRequest[T any](client requestDoer, request *http.Request) (T, error) {
	var result T

	response, err := client.Do(request)
	if err != nil {
		return result, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return result, err
	}

	if response.StatusCode != http.StatusOK {
		truncatedBody, _ := limitStringLength(string(body), 256)

		return result, fmt.Errorf(
			"unexpected status code %d for %s, response: %s",
			response.StatusCode,
			request.URL,
			truncatedBody,
		)
	}

	err = json.Unmarshal(body, &result)
	if err != nil {
		return result, err
	}

	return result, nil
}

func decodeJsonFromRequestTask[T any](client requestDoer) func(*http.Request) (T, error) {
	return func(request *http.Request) (T, error) {
		return decodeJsonFromRequest[T](client, request)
	}
}

// TODO: tidy up, these are a copy of the above but with a line changed
func decodeXmlFromRequest[T any](client requestDoer, request *http.Request) (T, error) {
	var result T

	response, err := client.Do(request)
	if err != nil {
		return result, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return result, err
	}

	if response.StatusCode != http.StatusOK {
		truncatedBody, _ := limitStringLength(string(body), 256)

		return result, fmt.Errorf(
			"unexpected status code %d for %s, response: %s",
			response.StatusCode,
			request.URL,
			truncatedBody,
		)
	}

	err = xml.Unmarshal(body, &result)
	if err != nil {
		return result, err
	}

	return result, nil
}

func decodeXmlFromRequestTask[T any](client requestDoer) func(*http.Request) (T, error) {
	return func(request *http.Request) (T, error) {
		return decodeXmlFromRequest[T](client, request)
	}
}

type workerPoolTask[I any, O any] struct {
	index  int
	input  I
	output O
	err    error
}

type workerPoolJob[I any, O any] struct {
	data    []I
	workers int
	task    func(I) (O, error)
	ctx     context.Context
}

const defaultNumWorkers = 10

func (job *workerPoolJob[I, O]) withWorkers(workers int) *workerPoolJob[I, O] {
	if workers == 0 {
		job.workers = defaultNumWorkers
	} else if workers > len(job.data) {
		job.workers = len(job.data)
	} else {
		job.workers = workers
	}

	return job
}

// func (job *workerPoolJob[I, O]) withContext(ctx context.Context) *workerPoolJob[I, O] {
// 	if ctx != nil {
// 		job.ctx = ctx
// 	}

// 	return job
// }

func newJob[I any, O any](task func(I) (O, error), data []I) *workerPoolJob[I, O] {
	return &workerPoolJob[I, O]{
		workers: defaultNumWorkers,
		task:    task,
		data:    data,
		ctx:     context.Background(),
	}
}

func workerPoolDo[I any, O any](job *workerPoolJob[I, O]) ([]O, []error, error) {
	results := make([]O, len(job.data))
	errs := make([]error, len(job.data))

	if len(job.data) == 0 {
		return results, errs, nil
	}

	tasksQueue := make(chan *workerPoolTask[I, O])
	resultsQueue := make(chan *workerPoolTask[I, O])

	var wg sync.WaitGroup

	for range job.workers {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for t := range tasksQueue {
				t.output, t.err = job.task(t.input)
				resultsQueue <- t
			}
		}()
	}

	var err error

	go func() {
	loop:
		for i := range job.data {
			select {
			default:
				tasksQueue <- &workerPoolTask[I, O]{
					index: i,
					input: job.data[i],
				}
			case <-job.ctx.Done():
				err = job.ctx.Err()
				break loop
			}
		}

		close(tasksQueue)
		wg.Wait()
		close(resultsQueue)
	}()

	for task := range resultsQueue {
		errs[task.index] = task.err
		results[task.index] = task.output
	}

	return results, errs, err
}
