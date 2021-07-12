package component

type TaskLaunchInfo struct {
	OciUrl               string
	EnvironmentVariables map[string]string
	Arguments            []string
}
