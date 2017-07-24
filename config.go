package phpkeeper

// 配置
type Config struct {
	Process string
	Dir string
	Args []string
	Env map[string] string
	Count int
	Server struct {
		Task string
		Process string
	}
}
