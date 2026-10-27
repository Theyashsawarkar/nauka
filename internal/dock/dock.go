// Package dock talks to Docker through its own command line (docker, and
// docker compose), so it follows your docker context, permissions and
// config exactly as the docker command does.
package dock

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Container is one container, with what docker inspect says about it.
type Container struct {
	ID, Name, Image, ImageID, State, Status, Health string
	Ports                                           string // docker ps's summary
	Created, Started                                time.Time
	Project, Service, WorkDir, ConfigFiles          string // compose labels
	Restart                                         string
	Cmd                                             string
	Env                                             []string
	Mounts                                          []Mount
	Networks                                        []NetLink
	Published                                       []Published
	Labels                                          map[string]string
	Raw                                             json.RawMessage // docker inspect, for the Inspect tab
}

// Running says whether it runs (paused counts).
func (c Container) Running() bool { return c.State == "running" || c.State == "paused" }

// Mount is a volume or folder mounted into a container.
type Mount struct {
	Type, Name, Source, Dest string
	RW                       bool
}

// NetLink is a container's place on a network.
type NetLink struct {
	Network, IP string
	Aliases     []string
}

// Published is a container port published on the host.
type Published struct {
	HostIP            string
	HostPort, CtrPort int
	Proto             string
}

// Stats is a container's live use.
type Stats struct {
	CPU                  float64 // percent
	Mem, MemLimit        string
	MemPerc              float64
	NetIO, BlockIO, PIDs string
}

// Image is a local image.
type Image struct {
	ID, Repo, Tag, Size, Created string
	CreatedAt                    time.Time
	Dangling                     bool
}

// Ref is how to name it: "postgres:16", or the short ID when untagged.
func (i Image) Ref() string {
	if i.Dangling {
		return shortID(i.ID)
	}
	return i.Repo + ":" + i.Tag
}

// Volume is a named or anonymous volume.
type Volume struct {
	Name, Driver, Mountpoint string
	Size                     string // from docker system df -v, "" until known
	Anonymous                bool
	Labels                   map[string]string
	Created                  string
}

// Network is a Docker network and who is on it.
type Network struct {
	ID, Name, Driver, Scope, Subnet, Gateway string
	Internal                                 bool
	Containers                               []NetMember
	Project                                  string
}

// Builtin says whether Docker made it (bridge, host, none): those can't
// be removed.
func (n Network) Builtin() bool { return n.Name == "bridge" || n.Name == "host" || n.Name == "none" }

// NetMember is a container on a network.
type NetMember struct{ Name, IP string }

// Usage is a line of docker system df.
type Usage struct {
	Type, Total, Active, Size, Reclaimable string
}

// Project is a compose project, from its containers' labels.
type Project struct {
	Name, WorkDir string
	ConfigFiles   []string
	Running, All  int
}

// Snapshot is everything at one moment.
type Snapshot struct {
	Containers []Container
	Images     []Image
	Volumes    []Volume
	Networks   []Network
	Projects   []Project
	Version    string
	At         time.Time
}

// ErrUnreachable wraps docker's own words when it can't reach the daemon.
type ErrUnreachable struct{ Msg string }

func (e ErrUnreachable) Error() string { return e.Msg }

// Run runs docker with args and returns its stdout; a failure carries
// docker's stderr.
func Run(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", args...)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return out, fmt.Errorf("%s", firstLine(msg))
	}
	return out, nil
}

func lines(b []byte) [][]byte {
	var out [][]byte
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		if l := bytes.TrimSpace(sc.Bytes()); len(l) > 0 {
			out = append(out, append([]byte(nil), l...))
		}
	}
	return out
}

// Load reads containers, images, volumes and networks.
func Load(ctx context.Context) (Snapshot, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return Snapshot{}, ErrUnreachable{"docker isn't installed"}
	}
	s := Snapshot{At: time.Now()}
	var err error
	if s.Containers, err = containers(ctx); err != nil {
		return s, ErrUnreachable{err.Error()}
	}
	s.Images, _ = images(ctx)
	s.Volumes, _ = volumes(ctx)
	s.Networks, _ = networks(ctx)
	s.Projects = projects(s.Containers)
	return s, nil
}

func containers(ctx context.Context) ([]Container, error) {
	out, err := Run(ctx, "ps", "-aq", "--no-trunc")
	if err != nil {
		return nil, err
	}
	ids := strings.Fields(string(out))
	if len(ids) == 0 {
		return nil, nil
	}
	raw, err := Run(ctx, append([]string{"inspect"}, ids...)...)
	if err != nil {
		return nil, err
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, err
	}
	var cs []Container
	for _, it := range items {
		var in struct {
			ID, Name, Image, Created string
			State                    struct {
				Status, StartedAt string
				Health            *struct{ Status string }
			}
			Config struct {
				Image      string
				Env        []string
				Cmd        []string
				Entrypoint []string
				Labels     map[string]string
			}
			HostConfig struct {
				RestartPolicy struct{ Name string }
			}
			Mounts []struct {
				Type, Name, Source, Destination string
				RW                              bool
			}
			NetworkSettings struct {
				Networks map[string]struct {
					IPAddress string
					Aliases   []string
				}
				Ports map[string][]struct{ HostIp, HostPort string }
			}
		}
		if json.Unmarshal(it, &in) != nil {
			continue
		}
		c := Container{ID: in.ID, Name: strings.TrimPrefix(in.Name, "/"), Image: in.Config.Image, ImageID: in.Image,
			State: in.State.Status, Restart: in.HostConfig.RestartPolicy.Name, Env: in.Config.Env, Labels: in.Config.Labels, Raw: it}
		if in.State.Health != nil {
			c.Health = in.State.Health.Status
		}
		c.Created, _ = time.Parse(time.RFC3339Nano, in.Created)
		c.Started, _ = time.Parse(time.RFC3339Nano, in.State.StartedAt)
		c.Cmd = strings.Join(append(in.Config.Entrypoint, in.Config.Cmd...), " ")
		l := in.Config.Labels
		c.Project, c.Service = l["com.docker.compose.project"], l["com.docker.compose.service"]
		c.WorkDir, c.ConfigFiles = l["com.docker.compose.project.working_dir"], l["com.docker.compose.project.config_files"]
		for _, m := range in.Mounts {
			c.Mounts = append(c.Mounts, Mount{Type: m.Type, Name: m.Name, Source: m.Source, Dest: m.Destination, RW: m.RW})
		}
		for name, n := range in.NetworkSettings.Networks {
			c.Networks = append(c.Networks, NetLink{Network: name, IP: n.IPAddress, Aliases: n.Aliases})
		}
		sort.Slice(c.Networks, func(i, j int) bool { return c.Networks[i].Network < c.Networks[j].Network })
		for port, binds := range in.NetworkSettings.Ports {
			num, proto, _ := strings.Cut(port, "/")
			cp, _ := strconv.Atoi(num)
			for _, b := range binds {
				hp, _ := strconv.Atoi(b.HostPort)
				if b.HostIp == "::" && hasHostPort(c.Published, hp) {
					continue
				}
				c.Published = append(c.Published, Published{HostIP: b.HostIp, HostPort: hp, CtrPort: cp, Proto: proto})
			}
		}
		sort.Slice(c.Published, func(i, j int) bool { return c.Published[i].HostPort < c.Published[j].HostPort })
		c.Status = statusLine(c)
		cs = append(cs, c)
	}
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].Running() != cs[j].Running() {
			return cs[i].Running()
		}
		if cs[i].Project != cs[j].Project {
			return cs[i].Project < cs[j].Project
		}
		return cs[i].Name < cs[j].Name
	})
	return cs, nil
}

func hasHostPort(ps []Published, port int) bool {
	for _, p := range ps {
		if p.HostPort == port {
			return true
		}
	}
	return false
}

func statusLine(c Container) string {
	switch c.State {
	case "running":
		s := "up " + Ago(c.Started)
		if c.Health != "" {
			s += " · " + c.Health
		}
		return s
	case "paused":
		return "paused"
	case "exited":
		code := ""
		var in struct{ State struct{ ExitCode int } }
		if json.Unmarshal(c.Raw, &in) == nil {
			code = " " + strconv.Itoa(in.State.ExitCode)
		}
		return "exited" + code
	}
	return c.State
}

func images(ctx context.Context) ([]Image, error) {
	out, err := Run(ctx, "images", "--no-trunc", "--format", "{{json .}}")
	if err != nil {
		return nil, err
	}
	var is []Image
	for _, l := range lines(out) {
		var in struct{ ID, Repository, Tag, Size, CreatedSince, CreatedAt string }
		if json.Unmarshal(l, &in) != nil {
			continue
		}
		im := Image{ID: in.ID, Repo: in.Repository, Tag: in.Tag, Size: in.Size, Created: in.CreatedSince,
			Dangling: in.Repository == "<none>" || in.Tag == "<none>"}
		im.CreatedAt, _ = time.Parse("2006-01-02 15:04:05 -0700 MST", in.CreatedAt)
		is = append(is, im)
	}
	sort.SliceStable(is, func(i, j int) bool {
		if is[i].Dangling != is[j].Dangling {
			return !is[i].Dangling
		}
		return is[i].CreatedAt.After(is[j].CreatedAt)
	})
	return is, nil
}

func volumes(ctx context.Context) ([]Volume, error) {
	out, err := Run(ctx, "volume", "ls", "-q")
	if err != nil {
		return nil, err
	}
	names := strings.Fields(string(out))
	if len(names) == 0 {
		return nil, nil
	}
	raw, err := Run(ctx, append([]string{"volume", "inspect"}, names...)...)
	if err != nil {
		return nil, err
	}
	var in []struct {
		Name, Driver, Mountpoint, CreatedAt string
		Labels                              map[string]string
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, err
	}
	var vs []Volume
	for _, v := range in {
		_, anon := v.Labels["com.docker.volume.anonymous"]
		vs = append(vs, Volume{Name: v.Name, Driver: v.Driver, Mountpoint: v.Mountpoint, Labels: v.Labels, Created: v.CreatedAt,
			Anonymous: anon || (len(v.Name) == 64 && isHex(v.Name))})
	}
	sort.Slice(vs, func(i, j int) bool {
		if vs[i].Anonymous != vs[j].Anonymous {
			return !vs[i].Anonymous
		}
		return vs[i].Name < vs[j].Name
	})
	return vs, nil
}

func networks(ctx context.Context) ([]Network, error) {
	out, err := Run(ctx, "network", "ls", "-q")
	if err != nil {
		return nil, err
	}
	ids := strings.Fields(string(out))
	if len(ids) == 0 {
		return nil, nil
	}
	raw, err := Run(ctx, append([]string{"network", "inspect"}, ids...)...)
	if err != nil {
		return nil, err
	}
	var in []struct {
		ID, Name, Driver, Scope string
		Internal                bool
		IPAM                    struct {
			Config []struct{ Subnet, Gateway string }
		}
		Containers map[string]struct{ Name, IPv4Address string }
		Labels     map[string]string
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, err
	}
	var ns []Network
	for _, n := range in {
		nw := Network{ID: n.ID, Name: n.Name, Driver: n.Driver, Scope: n.Scope, Internal: n.Internal, Project: n.Labels["com.docker.compose.project"]}
		if len(n.IPAM.Config) > 0 {
			nw.Subnet, nw.Gateway = n.IPAM.Config[0].Subnet, n.IPAM.Config[0].Gateway
		}
		for _, c := range n.Containers {
			ip, _, _ := strings.Cut(c.IPv4Address, "/")
			nw.Containers = append(nw.Containers, NetMember{Name: c.Name, IP: ip})
		}
		sort.Slice(nw.Containers, func(i, j int) bool { return nw.Containers[i].Name < nw.Containers[j].Name })
		ns = append(ns, nw)
	}
	sort.Slice(ns, func(i, j int) bool {
		if ns[i].Builtin() != ns[j].Builtin() {
			return !ns[i].Builtin()
		}
		return ns[i].Name < ns[j].Name
	})
	return ns, nil
}

func projects(cs []Container) []Project {
	by := map[string]*Project{}
	var order []string
	for _, c := range cs {
		if c.Project == "" {
			continue
		}
		p := by[c.Project]
		if p == nil {
			p = &Project{Name: c.Project, WorkDir: c.WorkDir}
			if c.ConfigFiles != "" {
				p.ConfigFiles = strings.Split(c.ConfigFiles, ",")
			}
			by[c.Project] = p
			order = append(order, c.Project)
		}
		p.All++
		if c.Running() {
			p.Running++
		}
	}
	sort.Strings(order)
	var out []Project
	for _, n := range order {
		out = append(out, *by[n])
	}
	return out
}

// LoadStats reads every running container's live use (takes a second or
// two: docker samples it).
func LoadStats(ctx context.Context) (map[string]Stats, error) {
	out, err := Run(ctx, "stats", "--no-stream", "--no-trunc", "--format", "{{json .}}")
	if err != nil {
		return nil, err
	}
	st := map[string]Stats{}
	for _, l := range lines(out) {
		var in struct{ ID, CPUPerc, MemUsage, MemPerc, NetIO, BlockIO, PIDs string }
		if json.Unmarshal(l, &in) != nil {
			continue
		}
		s := Stats{NetIO: in.NetIO, BlockIO: in.BlockIO, PIDs: in.PIDs}
		s.CPU, _ = strconv.ParseFloat(strings.TrimSuffix(in.CPUPerc, "%"), 64)
		s.MemPerc, _ = strconv.ParseFloat(strings.TrimSuffix(in.MemPerc, "%"), 64)
		s.Mem, s.MemLimit, _ = strings.Cut(in.MemUsage, " / ")
		st[in.ID] = s
	}
	return st, nil
}

// LoadUsage reads docker system df: totals per kind, and each volume's
// size (volumes have no size anywhere else).
func LoadUsage(ctx context.Context) ([]Usage, map[string]string, error) {
	out, err := Run(ctx, "system", "df", "--format", "{{json .}}")
	if err != nil {
		return nil, nil, err
	}
	var us []Usage
	for _, l := range lines(out) {
		var in struct{ Type, TotalCount, Active, Size, Reclaimable string }
		if json.Unmarshal(l, &in) == nil {
			us = append(us, Usage{in.Type, in.TotalCount, in.Active, in.Size, in.Reclaimable})
		}
	}
	sizes := map[string]string{}
	if v, err := Run(ctx, "system", "df", "-v", "--format", "{{json .}}"); err == nil {
		var in struct {
			Volumes []struct{ Name, Size string }
		}
		if json.Unmarshal(bytes.TrimSpace(v), &in) == nil {
			for _, vol := range in.Volumes {
				sizes[vol.Name] = vol.Size
			}
		}
	}
	return us, sizes, nil
}

// Version is the Docker engine's version.
func Version(ctx context.Context) string {
	out, err := Run(ctx, "version", "--format", "{{.Server.Version}}")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Logs reads a container's last lines, with timestamps.
func Logs(ctx context.Context, id string, tail int) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "docker", "logs", "-t", "--tail", strconv.Itoa(tail), id)
	out, err := cmd.CombinedOutput() // a container's stderr is half its log
	if err != nil && len(out) == 0 {
		return nil, err
	}
	s := strings.ReplaceAll(string(out), "\r", "")
	return strings.Split(strings.TrimRight(s, "\n"), "\n"), nil
}

// ImageDetails is what docker image inspect and history say.
type ImageDetails struct {
	Entrypoint, Platform string
	Exposed, Volumes     []string
	Tags                 []string
	Layers               []string // history: size and the step that made it
	Size                 int64
}

// InspectImage reads an image's details.
func InspectImage(ctx context.Context, id string) (ImageDetails, error) {
	var d ImageDetails
	raw, err := Run(ctx, "image", "inspect", id)
	if err != nil {
		return d, err
	}
	var in []struct {
		RepoTags     []string
		Architecture string
		Os           string
		Size         int64
		Config       struct {
			Entrypoint, Cmd []string
			ExposedPorts    map[string]any
			Volumes         map[string]any
		}
	}
	if json.Unmarshal(raw, &in) != nil || len(in) == 0 {
		return d, fmt.Errorf("no such image")
	}
	i := in[0]
	d.Tags, d.Size, d.Platform = i.RepoTags, i.Size, i.Os+"/"+i.Architecture
	d.Entrypoint = strings.Join(append(i.Config.Entrypoint, i.Config.Cmd...), " ")
	for p := range i.Config.ExposedPorts {
		d.Exposed = append(d.Exposed, p)
	}
	for v := range i.Config.Volumes {
		d.Volumes = append(d.Volumes, v)
	}
	sort.Strings(d.Exposed)
	sort.Strings(d.Volumes)
	if h, err := Run(ctx, "history", "--no-trunc", "--format", "{{.Size}}\t{{.CreatedBy}}", id); err == nil {
		for _, l := range strings.Split(strings.TrimSpace(string(h)), "\n") {
			size, by, _ := strings.Cut(l, "\t")
			by = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(by, "/bin/sh -c "), "#(nop) "))
			d.Layers = append(d.Layers, fmt.Sprintf("%8s  %s", size, by))
		}
	}
	return d, nil
}

// Files lists a folder inside a running container.
func Files(ctx context.Context, id, dir string) ([]File, error) {
	out, err := Run(ctx, "exec", id, "ls", "-la", dir)
	if err != nil {
		return nil, err
	}
	var fs []File
	for _, l := range strings.Split(string(out), "\n") {
		f := strings.Fields(l)
		if len(f) < 9 || strings.HasPrefix(l, "total") {
			continue
		}
		name := strings.Join(f[8:], " ")
		if name == "." || name == ".." {
			continue
		}
		if i := strings.Index(name, " -> "); i >= 0 {
			name = name[:i]
		}
		fs = append(fs, File{Name: name, Mode: f[0], Size: f[4], Dir: f[0][0] == 'd', Link: f[0][0] == 'l'})
	}
	sort.SliceStable(fs, func(i, j int) bool {
		if fs[i].Dir != fs[j].Dir {
			return fs[i].Dir
		}
		return fs[i].Name < fs[j].Name
	})
	return fs, nil
}

// File is an entry of a folder inside a container.
type File struct {
	Name, Mode, Size string
	Dir, Link        bool
}

// Ago is a short "how long": 3d, 5h, 12m.
func Ago(t time.Time) string {
	if t.IsZero() || t.Year() < 2000 {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

func shortID(id string) string {
	id = strings.TrimPrefix(id, "sha256:")
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// ShortID is the 12-character form of an ID.
func ShortID(id string) string { return shortID(id) }

func isHex(s string) bool {
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
