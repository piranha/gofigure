package main

import (
	"flag"
	"fmt"
	"http"
	"time"
	"strings"
	"os"
	"net"
	"bufio"
	"sort"
	)

var reqs *int = flag.Int("n", 1, "number of requests to make")
var concurrency *int = flag.Int("c", 1, "concurrency")

type someError struct {
	what string
	str  string
}

func (e *someError) String() string {
	return fmt.Sprintf("%s %q", e.what, e.str)
}

type result struct {
	time int64
	err os.Error
}

type Int64Array []int64
func (p Int64Array) Len() int           { return len(p) }
func (p Int64Array) Less(i, j int) bool { return p[i] < p[j] }
func (p Int64Array) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func main() {
	flag.Parse()

	if len(flag.Args()) == 0 {
		flag.Usage()
		return
	}

	url, err := getURL(flag.Arg(0))
	if err != nil {
		fmt.Printf("url is invalid: %s", err)
		return
	}

	ch := make(chan result, *reqs)
	results := make([]result, *reqs)
	running, i, j := 0, 0, 0

	now := time.Nanoseconds()
	for {
		if running < *concurrency && i < *reqs {
			go send(url, ch)
			running++
			i++
		} else if j < *reqs {
			results[j] = <- ch
			j++
			running--
		}

		if i == j && j >= *reqs {
			break
		}
	}

	printStats(results, time.Nanoseconds() - now)
}

func printStats(results []result, workTime int64) {
	times := make(Int64Array, 0)
	total := int64(0)

	for _, r := range(results) {
		if r.err == nil {
			times = append(times, r.time)
			total += r.time
		}
	}
	sort.Sort(times)

	fmt.Printf(`Statistics for request to %s

Time taken for tests:           %.3f ms
Average request takes:          %.3f ms
Median request time:            %.3f ms
Average time between responses: %.3f ms
Total failures:                 %d
`,
		flag.Arg(0),
		ms(workTime),
		ms(total / int64(len(times))),
		ms(times[len(times) / 2]),
		ms(workTime / int64(*reqs)),
		*reqs - len(times))
}

func ms(x int64) (float64) {
	return float64(x) / 1000000
}

func hasPort(s string) bool {
	return strings.LastIndex(s, ":") > strings.LastIndex(s, "]")
}

func getURL(url string) (*http.URL, os.Error) {
	URL, err := http.ParseURL(url)
	if err != nil {
		return nil, err
	}

	if URL.Scheme != "http" {
		return nil, &someError{"unsupported protocol scheme: %s", URL.Scheme}
	}

	if !hasPort(URL.Host) {
		URL.Host += ":http"
	}

	return URL, nil
}

func send(url *http.URL, out chan result) {
	var req http.Request
	req.URL = url
	rerr := func (err os.Error) result { return result{0, err} }

	now := time.Nanoseconds()
	conn, err := net.Dial("tcp", "", req.URL.Host)
	if err != nil {
		out <- rerr(err)
		return
	}

	err = req.Write(conn)
	if err != nil {
		conn.Close()
		out <- rerr(err)
		return
	}

	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, req.Method)
	out <- result{time.Nanoseconds() - now, err}

	conn.Close()
	resp.Body.Close()
}
