package chainapi

// TxSubmitPolicy 只描述固定顺序的提交路由，不引入权重和外部配置。
// 业务方可以在初始化阶段直接写死这份结构，作为主用与接盘顺序。
type TxSubmitPolicy struct {
	Routes []Route `json:"routes"`
}
