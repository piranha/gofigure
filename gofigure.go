package main

import (
	"bufio"
	"errors"
	"fmt"
	goopt "github.com/droundy/goopt"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"sort"
	"strings"
	"time"
)

var Author = "Alexander Solovyov"
var Version = "0.2"
var Summary = "gofigure [OPTS] URL\n"

var reqs = goopt.Int([]string{"-n", "--requests"}, 1,
	"number of requests to make")
var concurrency = goopt.Int([]string{"-c", "--concurrency"}, 1,
	"concurrency level")
var timeout = goopt.Int([]string{"-t", "--timeout"}, 1000,
	"timeout of each request in milliseconds")
var cpus = goopt.Int([]string{"-p", "--cpus"}, 0,
	"how many processes to run (0 - default)")

type someError struct {
	what string
	str  string
}

func (e *someError) Error() string {
	return fmt.Sprintf(e.what, e.str)
}

type result struct {
	time time.Duration
	err  error
}

type DurationArray []time.Duration

func (p DurationArray) Len() int {
	return len(p)
}
func (p DurationArray) Less(i, j int) bool {
	return p[i].Nanoseconds() < p[j].Nanoseconds()
}
func (p DurationArray) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func main() {
	goopt.Author = Author
	goopt.Version = Version
	goopt.Summary = Summary
	goopt.Parse(nil)

	if len(goopt.Args) == 0 {
		println(goopt.Usage())
		return
	}

	if *concurrency > *reqs {
		fmt.Printf("You can't have concurrency higher than number of requests\n")
		return
	}

	url, ip, err := getURL(goopt.Args[0])
	if err != nil {
		fmt.Printf("url is invalid: %s\n", err)
		return
	}

	runtime.GOMAXPROCS(*cpus)

	fmt.Printf("Statistics for requests to %s\n", goopt.Args[0])
	results, total := start(url, ip, *reqs, *concurrency)
	printStats(results, total)
}

func start(url *url.URL, addr string, requests int, concurrency int) ([]result, time.Duration) {
	results := make([]result, requests)
	queue := make(chan int, requests)
	out := make(chan result, concurrency)
	const fmtCompleted = "\rCompleted %d from %d requests"

	for i := 0; i < requests; i++ {
		queue <- i
	}

	now := time.Now()

	for i := 0; i < concurrency; i++ {
		go sender(url, addr, queue, out)
	}

	for i := 0; i < requests; i++ {
		results[i] = <-out

		if i > 0 && i%100 == 0 {
			fmt.Printf(fmtCompleted, i, requests)
		}
	}

	// erase 'Completed ...' line
	fmt.Printf("\r%*s", len(fmtCompleted)+10, " ")
	return results, time.Now().Sub(now)
}

func printStats(results []result, workTime time.Duration) {
	times := make(DurationArray, 0)
	total := time.Duration(0)

	for _, r := range results {
		if r.err == nil {
			times = append(times, r.time)
			total += r.time
		}
	}
	sort.Sort(times)

	average := time.Duration(0)
	median := time.Duration(0)
	if len(times) > 0 {
		average = time.Duration(total.Nanoseconds() / int64(len(times)))
		median = times[len(times) / 2]
	}

	fmt.Printf(`
Total requests performed:       %d
Total failures:                 %d
Time taken for tests:           %s
Average request takes:          %s
Median request time:            %s
Average time between responses: %s
Average requests per second:    %.3f
`,
		*reqs,
		*reqs-len(times),
		workTime,
		average,
		median,
		time.Duration(workTime.Nanoseconds() / int64(*reqs)),
		(float64(*reqs) / workTime.Seconds()))
}

func hasPort(s string) bool {
	return strings.LastIndex(s, ":") > strings.LastIndex(s, "]")
}

func getURL(rawurl string) (*url.URL, string, error) {
	URL, err := url.Parse(rawurl)
	if err != nil {
		return nil, "", err
	}

	if URL.Scheme != "http" && URL.Scheme != "https" {
		return nil, "", &someError{"unsupported protocol scheme: %s", URL.Scheme}
	}

	bits := strings.Split(URL.Host, ":")
	host := bits[0]
	var port string
	if len(bits) > 1 {
		port = bits[1]
	} else {
		port = "80"
	}

	ip, err := net.ResolveIPAddr("ip", host)
	if err != nil {
		return nil, "", err
	}

	return URL, ip.String() + ":" + port, nil
}

func sender(url *url.URL, addr string, queue chan int, out chan result) {
	for _ = range queue {
		out <- send(url, addr)
	}
}

type respErr struct {
	resp *http.Response
	err  error
}

func send(url *url.URL, addr string) result {
	var req http.Request
	req.URL = url

	now := time.Now()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return result{0, err}
	}

	err = req.Write(conn)
	if err != nil {
		conn.Close()
		return result{0, err}
	}

	ch := make(chan respErr, 1)
	go func() {
		reader := bufio.NewReader(conn)
		response, err := http.ReadResponse(reader, &req)
		ch <- respErr{response, err}
	}()

	var res result

	select {
	case <-time.After(time.Duration(*timeout * 1e6)):
		res = result{time.Now().Sub(now), errors.New("Timeout!")}
	case rerr := <-ch:
		res = result{time.Now().Sub(now), rerr.err}
		conn.Close()
		rerr.resp.Body.Close()
	}

	return res
}
