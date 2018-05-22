package main

import (
	"encoding/json"
	"fmt"
	vault "github.com/akkeris/vault-client"
	"gopkg.in/Shopify/sarama.v1"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

var total int
var alamoapiusername string
var alamoapipassword string

type Plan struct {
	Name      string `json:"name"`
	Resources struct {
		Requests struct {
			Memory string `json:"memory"`
		} `json:"requests"`
		Limits struct {
			Memory string `json:"memory"`
		} `json:"limits"`
	} `json:"resources"`
	Price int `json:"price"`
}

type SimpleStat struct {
	PodName         string `json:"pod_name"`
	ContainerName   string `json:"container_name"`
	Namespace       string `json:"namespace"`
	MemoryTotal     string `json:"memory_total_s"`
	MemoryRSS       string `json:"memory_rss_s"`
	MemoryCache     string `json:"memory_cache_s"`
	MemorySwap      string `json:"memory_swap_s"`
	FileSystemUsage string `json:"container_fs_usage_bytes"`
}

type PromStat struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric struct {
				ContainerName string `json:"container_name"`
				Namespace     string `json:"namespace"`
				PodName       string `json:"pod_name"`
			} `json:"metric"`
			Value []interface{} `json:"value"`
		} `json:"result"`
	} `json:"data"`
}

type Logspec struct {
	Log    string    `json:"log"`
	Stream string    `json:"stream"`
	Time   time.Time `json:"time"`
	Docker struct {
		ContainerID string `json:"container_id"`
	} `json:"docker"`
	Kubernetes struct {
		NamespaceName string `json:"namespace_name"`
		PodID         string `json:"pod_id"`
		PodName       string `json:"pod_name"`
		ContainerName string `json:"container_name"`
		Labels        struct {
			Name string `json:"name"`
		} `json:"labels"`
		Host string `json:"host"`
	} `json:"kubernetes"`
	Topic string `json:"topic"`
	Tag   string `json:"tag"`
}

type Bindspec struct {
	App      string `json:"appname"`
	Space    string `json:"space"`
	Bindtype string `json:"bindtype"`
	Bindname string `json:"bindname"`
}

type Spaceappspec struct {
	Appname   string     `json:"appname"`
	Space     string     `json:"space"`
	Instances int        `json:"instances"`
	Bindings  []Bindspec `json:"bindings"`
	Plan      string     `json:"plan"`
}

type Spacelist struct {
	Spaces []string `json:"spaces"`
}

var limitlist map[string]float64
var planlimits map[string]float64
var producer sarama.SyncProducer

func main() {
	var brokersenv = os.Getenv("KAFKA_BROKERS")
	brokers := strings.Split(brokersenv, ",")
	var err error
	producer, err = sarama.NewSyncProducer(brokers, nil)
	if err != nil {
		fmt.Println(err)
	}
	limitlist = make(map[string]float64)
	planlimits = make(map[string]float64)
	setCreds()
	populatePlanLimits()
	populateLimits()
	getMetrics()
	delay, err := strconv.Atoi(os.Getenv("DELAY_MINUTES"))
	if err != nil {
		fmt.Println(err)
		fmt.Println("no delay set")
		os.Exit(1)
	}
	for i := 1; i <= 6; i++ {
		time.Sleep(time.Duration(delay) * time.Minute)
		getMetrics()
	}
	fmt.Println("done")
}

func populatePlanLimits() {

	var plans []Plan
	client := http.Client{}
	req, err := http.NewRequest("GET", os.Getenv("ALAMO_API_URL")+"/v1/apps/plans", nil)
	req.SetBasicAuth(alamoapiusername, alamoapipassword)

	if err != nil {
		fmt.Println(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()
	bb, _ := ioutil.ReadAll(resp.Body)
	fmt.Println(string(bb))
	err = json.Unmarshal(bb, &plans)
	if err != nil {
		fmt.Println(err)
	}
	for _, element := range plans {
		memstring := strings.Replace(element.Resources.Limits.Memory, "Mi", "", -1)
		f, err := strconv.ParseFloat(memstring, 64)
		if err != nil {
			fmt.Println(err)
		}
		planlimits[element.Name] = f
	}
	fmt.Println(planlimits)
}

func populateLimits() {
	fmt.Println("running addspaces")
	fmt.Println("top of the hour")
	spaces, err := getSpaces()
	fmt.Println(spaces)
	if err != nil {
		fmt.Println(err)
	}
	for _, space := range spaces.Spaces {
		processSpace(space)
	}
	fmt.Println(limitlist)
}

func processSpace(space string) {

	fmt.Println("processing " + space)
	apps, err := getApps(space)
	for _, app := range apps {
		if app.Plan != "noplan" {
			limitlist[app.Appname+"-"+space] = getLimit(app.Plan)
		}
	}
	if err != nil {
		fmt.Println(err)
	}
}
func getApps(space string) (a []Spaceappspec, e error) {
	var apps []Spaceappspec
	client := http.Client{}
	req, err := http.NewRequest("GET", os.Getenv("ALAMO_API_URL")+"/v1/space/"+space+"/apps", nil)
	req.SetBasicAuth(alamoapiusername, alamoapipassword)

	if err != nil {
		fmt.Println(err)
		return apps, err
	}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return apps, err
	}
	defer resp.Body.Close()
	bb, _ := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(bb, &apps)
	if err != nil {
		fmt.Println(err)
		return apps, err
	}
	return apps, nil
}

func getSpaces() (s Spacelist, e error) {
	var spaces Spacelist

	client := http.Client{}
	req, err := http.NewRequest("GET", os.Getenv("ALAMO_API_URL")+"/v1/spaces", nil)
	req.SetBasicAuth(alamoapiusername, alamoapipassword)
	if err != nil {
		fmt.Println(err)
		return spaces, err
	}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return spaces, err
	}
	defer resp.Body.Close()
	bb, _ := ioutil.ReadAll(resp.Body)

	err = json.Unmarshal(bb, &spaces)
	if err != nil {
		fmt.Println(err)
		return spaces, err
	}
	return spaces, nil
}

func getLimit(plan string) float64 {
	var toreturn float64
	toreturn = 256
	if val, ok := planlimits[plan]; ok {
		toreturn = val
	}
	return toreturn
}

func getMetrics() {

	var total int
	total = 0
	var metrics map[string]SimpleStat
	metrics = make(map[string]SimpleStat)
	metrics = addIn(metrics, "MemoryTotal", "container_memory_usage_bytes")
	metrics = addIn(metrics, "MemorySwap", "container_memory_swap")
	metrics = addIn(metrics, "MemoryRSS", "container_memory_rss")
	metrics = addIn(metrics, "MemoryCache", "container_memory_cache")
	metrics = addIn(metrics, "FileSystemUsage", "container_fs_usage_bytes")
	for _, v := range metrics {
		total = total + 1
		postMemorySample(v)
	}
}

func addIn(metrics map[string]SimpleStat, fieldname string, metric string) map[string]SimpleStat {
	client := http.Client{}
	prometheusurl := os.Getenv("PROMETHEUS_URL")
	req, err := http.NewRequest("GET", prometheusurl+"/api/v1/query?query=sum+by(pod_name,namespace,container_name)("+metric+"{container_name!=\"POD\",container_name!=\"\",namespace!=\"kube-system\",job=\"kubernetes-cadvisors\"})", nil)
	if err != nil {
		fmt.Println("error creating request")
		return nil
	}
	resp, doerr := client.Do(req)

	if doerr != nil {
		fmt.Println("error doing request")
		fmt.Println(doerr)
		return nil
	}

	defer resp.Body.Close()
	bodybytes, err := ioutil.ReadAll(resp.Body)
	var promstat PromStat
	err = json.Unmarshal(bodybytes, &promstat)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	for _, element := range promstat.Data.Result {
		_, exists := metrics[element.Metric.PodName]
		if !exists {
			var simplestat SimpleStat
			simplestat.PodName = element.Metric.PodName
			simplestat.Namespace = element.Metric.Namespace
			simplestat.ContainerName = element.Metric.ContainerName
			v := reflect.ValueOf(&simplestat).Elem().FieldByName(fieldname)
			if v.IsValid() {
				v.SetString(element.Value[1].(string))
			}
			metrics[element.Metric.PodName] = simplestat
		}
		if exists {
			simplestat := metrics[element.Metric.PodName]
			v := reflect.ValueOf(&simplestat).Elem().FieldByName(fieldname)
			if v.IsValid() {
				v.SetString(element.Value[1].(string))
			}
			metrics[element.Metric.PodName] = simplestat
		}
	}
	return metrics
}

func postMemorySample(simplestat SimpleStat) {
	var logentry Logspec
	var f float64
	f, _ = strconv.ParseFloat(simplestat.MemoryTotal, 64)
	if limit, ok := limitlist[simplestat.ContainerName+"-"+simplestat.Namespace]; ok {
		evaluateLimit(f, limit, simplestat)
	}
	memory_total := ByteFormat(f, 2)

	f, _ = strconv.ParseFloat(simplestat.MemoryRSS, 64)
	memory_rss := ByteFormat(f, 2)
	f, _ = strconv.ParseFloat(simplestat.MemoryCache, 64)
	memory_cache := ByteFormat(f, 2)
	f, _ = strconv.ParseFloat(simplestat.MemorySwap, 64)
	memory_swap := ByteFormat(f, 2)
	file_system_usage := simplestat.FileSystemUsage

	logentry.Log = "[metrics] dyno=" + simplestat.PodName + " sample#memory_total=" + memory_total + " sample#memory_rss=" + memory_rss + " sample#memory_cache=" + memory_cache + " sample#memory_swap=" + memory_swap + " sample#file_system_usage=" + file_system_usage + " tag#dyno=" + simplestat.PodName
	logentry.Stream = "stdout"
	logentry.Kubernetes.NamespaceName = simplestat.Namespace
	logentry.Kubernetes.PodName = simplestat.PodName
	logentry.Kubernetes.ContainerName = simplestat.ContainerName
	logentry.Kubernetes.Labels.Name = simplestat.ContainerName
	logentry.Topic = simplestat.Namespace
	logentry.Tag = "metrics"
	str, err := json.Marshal(logentry)
	if err != nil {
		fmt.Println("Error preparing request")
		return
	}
	jsonStr := string(str)
	msg := &sarama.ProducerMessage{Topic: logentry.Topic, Value: sarama.StringEncoder(jsonStr)}

	_, _, err = producer.SendMessage(msg)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println("message sent")
	}

}

func RoundUp(input float64, places int) (newVal float64) {
	var round float64
	pow := math.Pow(10, float64(places))
	digit := pow * input
	round = math.Ceil(digit)
	newVal = round / pow
	return
}

func ByteFormat(inputNum float64, precision int) string {

	if precision <= 0 {
		precision = 1
	}

	var unit string
	var returnVal float64

	if inputNum >= 1000000000000000000000000 {
		returnVal = RoundUp((inputNum / 1208925819614629174706176), precision)
		unit = "YB" // yottabyte
	} else if inputNum >= 1000000000000000000000 {
		returnVal = RoundUp((inputNum / 1180591620717411303424), precision)
		unit = "ZB" // zettabyte
	} else if inputNum >= 10000000000000000000 {
		returnVal = RoundUp((inputNum / 1152921504606846976), precision)
		unit = "EB" // exabyte
	} else if inputNum >= 1000000000000000 {
		returnVal = RoundUp((inputNum / 1125899906842624), precision)
		unit = "PB" // petabyte
	} else if inputNum >= 1000000000000 {
		returnVal = RoundUp((inputNum / 1099511627776), precision)
		unit = "TB" // terrabyte
	} else if inputNum >= 1000000000 {
		returnVal = RoundUp((inputNum / 1073741824), precision)
		unit = "GB" // gigabyte
	} else if inputNum >= 1000000 {
		returnVal = RoundUp((inputNum / 1048576), precision)
		unit = "MB" // megabyte
	} else if inputNum >= 1000 {
		returnVal = RoundUp((inputNum / 1024), precision)
		unit = "KB" // kilobyte
	} else {
		returnVal = inputNum
		unit = "bytes" // byte
	}

	return strconv.FormatFloat(returnVal, 'f', precision, 64) + unit

}

func evaluateLimit(used float64, limit float64, simplestat SimpleStat) {
	app := simplestat.ContainerName + "-" + simplestat.Namespace
	fmt.Println("checking limit of " + app)
	fmt.Println("current usage:")
	fmt.Println(used)
	fmt.Println("limit is:")
	fmt.Println(limit)
	limitbytes := limit * 1048576
	fmt.Println("limit in bytes is:")
	fmt.Println(limitbytes)
	fmt.Println("percentage is:")
	percentage := (used / limitbytes) * 100
	fmt.Println(percentage)
	if percentage > 80 && percentage < 90 {
		fmt.Println("limit warning")
		injectMemoryAlert("warning", simplestat, limit)
	}
	if percentage >= 90 && percentage < 100 {
		fmt.Println("limit critical")
		injectMemoryAlert("critical", simplestat, limit)
	}
	if percentage >= 100 {
		fmt.Println("limit exceeded")
		injectMemoryAlert("exceeded", simplestat, limit)
	}

}

func injectMemoryAlert(alerttype string, simplestat SimpleStat, limit float64) {
	f, _ := strconv.ParseFloat(simplestat.MemoryTotal, 64)
	valuestring := ByteFormat(f, 0)
	limitbytes := limit * 1048576
	limitstring := ByteFormat(limitbytes, 0)
	fmt.Println("Injecting Alert")
	fmt.Println(simplestat.ContainerName + "-" + simplestat.Namespace)
	fmt.Println(alerttype)
	var logentry Logspec
	if alerttype == "warning" {
		logentry.Log = "[alert] dyno=" + simplestat.PodName + " Error R14 (Memory limit warning) " + valuestring + "/" + limitstring
	} else {
		logentry.Log = "[alert] dyno=" + simplestat.PodName + " Error R15 (Memory limit critical) " + valuestring + "/" + limitstring
	}

	logentry.Stream = "stdout"
	logentry.Kubernetes.NamespaceName = simplestat.Namespace
	logentry.Kubernetes.PodName = simplestat.PodName
	logentry.Kubernetes.ContainerName = simplestat.ContainerName
	logentry.Kubernetes.Labels.Name = simplestat.ContainerName
	logentry.Topic = simplestat.Namespace
	logentry.Tag = "metrics"
	fmt.Println(logentry.Log)
	fmt.Println("**************** send log *****************")
	str, err := json.Marshal(logentry)
	if err != nil {
		fmt.Println("Error preparing request")
		return
	}
	jsonStr := string(str)

	fmt.Println(jsonStr)
	msg := &sarama.ProducerMessage{Topic: logentry.Topic, Value: sarama.StringEncoder(jsonStr)}

	_, _, err = producer.SendMessage(msg)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println("message sent")
	}

}

func setCreds() {

	alamoapiusername = vault.GetField("secret/ops/alamo/api", "username")
	alamoapipassword = vault.GetField("secret/ops/alamo/api", "password")

}
