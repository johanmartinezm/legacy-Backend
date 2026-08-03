package domain

type ArticleStat struct {
	ArticleID string `json:"article_id"`
	Title     string `json:"title"`
	Views     int    `json:"views"`
}

type UserStat struct {
	UserID    string `json:"user_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Reads     int    `json:"reads"`
}

type PeriodStats struct {
	Period string `json:"period"`
	Views  int    `json:"views"`
}

type DashboardStats struct {
	TopArticles  []ArticleStat `json:"top_articles"`
	TopUsers     []UserStat    `json:"top_users"`
	WeeklyStats  []PeriodStats `json:"weekly_stats"`
	MonthlyStats []PeriodStats `json:"monthly_stats"`
	YearlyStats  []PeriodStats `json:"yearly_stats"`
}
