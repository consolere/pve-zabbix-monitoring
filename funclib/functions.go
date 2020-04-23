package funclib

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/BurntSushi/toml"
	"github.com/jessevdk/go-flags"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var Logger *log.Logger

///////////////// CONFIG START/////////////
type Config struct {
	GLOBAL struct {
		CheckingTime int
		FileLogPath  string
		HostName     string
	}
	PVE PVE
}

type PVE struct {
	URL      string
	User     string
	Password string
}

func ReadConfig(f string) (*Config, error) {
	_, err := os.Stat(f)
	if err != nil {
		log.Fatal("Config file is missing: ", f)
	}
	config := new(Config)
	if _, err := toml.DecodeFile(f, &config); err != nil {
		log.Fatal("Config file is bad: ", f, "\n", err)
	}
	return config, nil
}

//////////////// AUTH ///////////////////

type Auth struct {
	Data struct {
		Ticket   string `json:"ticket"`
		Username string `json:"username"`
		Cap      struct {
			Nodes struct {
			} `json:"nodes"`
			Access struct {
			} `json:"access"`
			Storage struct {
			} `json:"storage"`
			Vms struct {
				VMAudit       int `json:"VM.Audit"`
				VMConfigCDROM int `json:"VM.Config.CDROM"`
				VMBackup      int `json:"VM.Backup"`
				VMPowerMgmt   int `json:"VM.PowerMgmt"`
				VMConsole     int `json:"VM.Console"`
			} `json:"vms"`
			Dc struct {
			} `json:"dc"`
		} `json:"cap"`
		CSRFPreventionToken string `json:"CSRFPreventionToken"`
	} `json:"data"`
}

func GetTicket(pve *PVE) (string, error) {

	auth := &Auth{}
	//var result []byte

	timeout := time.Duration(time.Duration(10) * time.Second)
	client := &http.Client{
		Timeout: timeout,
	}

	data := []byte(`username=` + pve.User + `&password=` + pve.Password)
	r := bytes.NewReader(data)
	req, err := http.NewRequest("POST", pve.URL+"/api2/json/access/ticket", r)
	//req, err := http.NewRequest("POST","https://192.168.0.209:8006" + "/api2/json/access/ticket", r)
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	req.Close = true
	//req.Header.Add("Accept", "application/json, text/plain, */*")

	resp, err := client.Do(req)
	check(err)

	body, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	err = json.Unmarshal(body, &auth)
	check(err)

	if auth.Data.Ticket == "" {
		Logger.Panic("Unable to login in PVE " + pve.URL)
	}

	return auth.Data.Ticket, nil
}

//////////////////////////  GET CLUSTER STAT INFO //////////////////

type ClusterShot struct {
	Data []*VM `json:"data"`
}

func GetClusterStatus(pve *PVE) (*ClusterShot, error) {

	ticket, err := GetTicket(pve)
	check(err)

	clusterShot := &ClusterShot{}

	timeout := time.Duration(time.Duration(10) * time.Second)
	client := &http.Client{
		Timeout: timeout,
	}

	//data := []byte(`username=` + pve.User + `&password=` + pve.Password)
	data := []byte(``)
	r := bytes.NewReader(data)
	req, err := http.NewRequest("GET", pve.URL+"/api2/json/cluster/resources", r)
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	req.AddCookie(&http.Cookie{Name: "PVEAuthCookie", Value: ticket})
	req.Close = true
	//req.Header.Add("Accept", "application/json, text/plain, */*")

	resp, err := client.Do(req)
	check(err)

	body, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	err = json.Unmarshal(body, &clusterShot)
	check(err)

	return clusterShot, nil
}

///////////////// DATA PROCESSING/////////////

type Node struct {
	Name  string
	VMs   []*VM
	Stat  *VM
	Stors []*VM
}

type VM struct {
	Maxdisk   int64   `json:"maxdisk"`
	Name      string  `json:"name,omitempty"`
	Type      string  `json:"type"`
	Uptime    int     `json:"uptime"`
	Status    string  `json:"status"`
	Netin     int64   `json:"netin,omitempty"`
	CPU       float64 `json:"cpu"`
	Mem       int64   `json:"mem"`
	Disk      int64   `json:"disk"`
	Node      string  `json:"node"`
	Vmid      int     `json:"vmid,omitempty"`
	Diskread  int64   `json:"diskread,omitempty"`
	Diskwrite int64   `json:"diskwrite,omitempty"`
	Maxcpu    int     `json:"maxcpu"`
	Netout    int64   `json:"netout,omitempty"`
	Maxmem    int64   `json:"maxmem"`
	ID        string  `json:"id"`
	Template  int     `json:"template,omitempty"`
	Level     string  `json:"level,omitempty"`
	Storage   string  `json:"storage,omitempty"`
	avCPU     int
	avMem     int64
	useMem    float64
	useDisk   float64
	diskIO    int64
	netIO     int64
}

type Cluster struct {
	Data map[string]*Node
}

func ProcessingData(cluster *ClusterShot) (*Cluster, error) {
	statData := Cluster{map[string]*Node{}}
	for _, i := range cluster.Data {

		if _, ok := statData.Data[i.Node]; !ok {
			statData.Data[i.Node] = &Node{i.Node, []*VM{}, &VM{}, []*VM{}}
		}

		switch i.Type {
		case "node":
			i.Name = i.Node
			*statData.Data[i.Node].Stat = *i
		case "storage":
			i.Name = "store:" + i.Storage
			statData.Data[i.Node].Stors = append(statData.Data[i.Node].Stors, i)
		default:
			statData.Data[i.Node].VMs = append(statData.Data[i.Node].VMs, i)
		}

	}

	for nodename, node := range statData.Data {

		vmsCPU := 0
		var vmsMem int64 = 0

		for i := 0; i < len(node.VMs); i++ {
			node.VMs[i].useMem = float64(node.VMs[i].Mem) * 100 / float64(node.VMs[i].Maxmem)
			node.VMs[i].useDisk = float64(node.VMs[i].Disk) * 100 / float64(node.VMs[i].Maxdisk)
			node.VMs[i].diskIO = node.VMs[i].Diskread + node.VMs[i].Diskwrite
			node.VMs[i].netIO = node.VMs[i].Netin + node.VMs[i].Netout
			vmsCPU += node.VMs[i].Maxcpu
			vmsMem += node.VMs[i].Maxmem
		}

		for i := 0; i < len(node.Stors); i++ {
			node.Stors[i].useDisk = float64(node.Stors[i].Disk) * 100 / float64(node.Stors[i].Maxdisk)
		}

		statData.Data[nodename].Stat.avCPU = statData.Data[nodename].Stat.Maxcpu - vmsCPU
		statData.Data[nodename].Stat.avMem = statData.Data[nodename].Stat.Maxmem - vmsMem
		statData.Data[nodename].Stat.useMem = float64(statData.Data[nodename].Stat.Mem) * 100 / float64(statData.Data[nodename].Stat.Maxmem)
		statData.Data[nodename].Stat.useDisk = float64(statData.Data[nodename].Stat.Disk) * 100 / float64(statData.Data[nodename].Stat.Maxdisk)

	}

	return &statData, nil

}

///////////////// PRINT //////////////////

func PrintStat(d *Cluster, t string) error {

	fmt.Println("")
	format := "%-28v%-10v%-9v%-8v%-8v%-8v%-9v%-8v%-7v%-7v%-8v%-6v%-11v\n"

	for _, v := range d.Data {
		//fmt.Printf("%v\t%v\n", strings.ToUpper(k), "----------------------------------------------------------------------------------------------------------")
		fmt.Printf("%v\n", "------------------------------------------------------------------------------------------------------------------------")

		fmt.Printf(format, "Name", "Status", "Uptime", "%CPU", "xCPU", "%Mem", "xMem", "%Disk", "xDisk", "diskIO", "netIO", "Type", "Vmid")
		fmt.Printf("%v\n", "------------------------------------------------------------------------------------------------------------------------")
		fmt.Printf(format, strings.ToUpper(v.Stat.Node), v.Stat.Status, strconv.Itoa(v.Stat.Uptime/86400)+"d", Round(v.Stat.CPU*100, 2), strconv.Itoa(v.Stat.Maxcpu)+"("+strconv.Itoa(v.Stat.avCPU)+")", Round(v.Stat.useMem, 2), fmt.Sprintf("%v(%v)", v.Stat.Maxmem/1073741824, v.Stat.avMem/1073741824), Round(v.Stat.useDisk, 2), v.Stat.Maxdisk/1073741824, (v.Stat.Diskread+v.Stat.Diskread)/1073741824, (v.Stat.Netout+v.Stat.Netin)/1073741824, v.Stat.Type, "")
		fmt.Printf("\n")

		for _, V := range v.VMs {
			fmt.Printf(format, V.Name, V.Status, strconv.Itoa(V.Uptime/86400)+"d", Round(V.CPU*100, 2), V.Maxcpu, Round(V.useMem, 2), V.Maxmem/1073741824, Round(V.useDisk, 2), V.Maxdisk/1073741824, (V.Diskread+V.Diskwrite)/1073741824, (V.Netin+V.Netout)/1073741824, V.Type, V.Vmid)
		}
		fmt.Printf("\n")

		for _, V := range v.Stors {
			fmt.Printf(format, V.Name, V.Status, "", "", "", "", "", Round(V.useDisk, 2), V.Maxdisk/1073741824, "", "", "", "")
		}

		fmt.Printf("\r\n\n")

		//Logger.Println("\n-----------------------------------------------------------------------------------------")
	}

	return nil
}

//////////////////////  ZABBIX   ////////////////////

var opts struct {
	Zabbix    bool `short:"z" long:"zabbix" description:"for Zabbix monitoring system"`
	Discovery bool `short:"d" long:"discovery" description:"discovery out for Zabbix monitoring system"`
	State     bool `short:"s" long:"state" description:"state for Zabbix monitoring system"`
}

type ZabbixDiscovery struct {
	VMs []*DiscoVM `json:"data"`
}

type DiscoVM struct {
	NodeName string `json:"{#NODENAME}"`
	VMName   string `json:"{#VMNAME}"`
}

func CliMode(d *Cluster, h string) error {
	_, err := flags.Parse(&opts)
	check(err)

	if opts.Zabbix && opts.Discovery {
		disco := new(ZabbixDiscovery)
		for nodename, node := range d.Data {
			for _, vm := range node.VMs {
				s := DiscoVM{nodename, vm.Name}
				disco.VMs = append(disco.VMs, &s)
			}
			s := DiscoVM{nodename, "node"}
			disco.VMs = append(disco.VMs, &s)
		}
		j, err := json.Marshal(disco)
		check(err)
		fmt.Println(string(j))
	} else {
		if opts.Zabbix && opts.State {
			fmt.Println("1")
			err := sendStateToZabbix(d, h)
			check(err)
		}
	}
	return nil
}

func sendStateToZabbix(d *Cluster, hostname string) error {
	strOut := ""

	for k, v := range d.Data {
		for _, V := range v.VMs {
			strOut = MakeStringForZabbix(strOut, k, V)
		}
		v.Stat.Name = v.Stat.Type
		strOut = MakeStringForZabbix(strOut, k, v.Stat)

	}

	subProcess := exec.Command("/usr/bin/zabbix_sender", "--config", "/etc/zabbix/zabbix_agentd.conf", "--host="+hostname, "--input-file", "-")

	stdin, err := subProcess.StdinPipe()
	check(err)
	defer stdin.Close()

	if err = subProcess.Start(); err != nil {
		Logger.Println("An error occured: ", err)
	}

	io.WriteString(stdin, strOut)

	return nil
}

func MakeStringForZabbix(strOut, k string, V *VM) string {

	str := "- pve.maxdisk[" + k + "." + V.Name + "] " + strconv.FormatInt(V.Maxdisk, 10)
	strOut = strOut + str + "\n"
	str = "- pve.netin[" + k + "." + V.Name + "] " + strconv.FormatInt(V.Netin, 10)
	strOut = strOut + str + "\n"
	str = "- pve.mem[" + k + "." + V.Name + "] " + strconv.FormatInt(V.Mem, 10)
	strOut = strOut + str + "\n"
	str = "- pve.disk[" + k + "." + V.Name + "] " + strconv.FormatInt(V.Disk, 10)
	strOut = strOut + str + "\n"
	str = "- pve.diskread[" + k + "." + V.Name + "] " + strconv.FormatInt(V.Diskread, 10)
	strOut = strOut + str + "\n"
	str = "- pve.diskwrite[" + k + "." + V.Name + "] " + strconv.FormatInt(V.Diskwrite, 10)
	strOut = strOut + str + "\n"
	str = "- pve.maxcpu[" + k + "." + V.Name + "] " + strconv.Itoa(V.Maxcpu)
	strOut = strOut + str + "\n"
	str = "- pve.netout[" + k + "." + V.Name + "] " + strconv.FormatInt(V.Netout, 10)
	strOut = strOut + str + "\n"
	str = "- pve.maxmem[" + k + "." + V.Name + "] " + strconv.FormatInt(V.Maxmem, 10)
	strOut = strOut + str + "\n"

	//---------
	str = "- pve.uptime[" + k + "." + V.Name + "] " + strconv.Itoa(V.Uptime)
	strOut = strOut + str + "\n"
	str = "- pve.cpu[" + k + "." + V.Name + "] " + fmt.Sprintf("%f", V.CPU)
	strOut = strOut + str + "\n"

	str = "- pve.usemem[" + k + "." + V.Name + "] " + fmt.Sprintf("%f", V.useMem)
	strOut = strOut + str + "\n"
	str = "- pve.usedisk[" + k + "." + V.Name + "] " + fmt.Sprintf("%f", V.useDisk)
	strOut = strOut + str + "\n"
	str = "- pve.diskio[" + k + "." + V.Name + "] " + strconv.FormatInt(V.diskIO, 10)
	strOut = strOut + str + "\n"
	str = "- pve.netio[" + k + "." + V.Name + "] " + strconv.FormatInt(V.netIO, 10)
	strOut = strOut + str + "\n"
	//---------

	return strOut
}

func Round(x float64, prec int) float64 {
	var rounder float64
	pow := math.Pow(10, float64(prec))
	intermed := x * pow
	_, frac := math.Modf(intermed)
	if frac >= 0.5 {
		rounder = math.Ceil(intermed)
	} else {
		rounder = math.Floor(intermed)
	}

	return rounder / pow
}

func check(e error) {
	if e != nil {
		Logger.Panic(e)
	}
}
