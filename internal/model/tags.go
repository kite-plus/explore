package model

// Tag is one entry of the list Explore files posts under.
type Tag struct {
	Slug  string // in URLs and the database
	ZH    string
	EN    string
	About string // what belongs under the tag, written for the tagger
}

// MaxTagsPerEntry is how many tags one post may carry.
const MaxTagsPerEntry = 3

// Tags is the tag list in display order. Changing it is a product decision,
// made in docs/design/accounts.md section 5.1; posts already filed keep their
// tags until they are tagged again.
var Tags = []Tag{
	{"frontend", "前端", "Frontend", "Web front-end work: HTML, CSS, JavaScript and TypeScript in the browser, UI frameworks such as React or Vue, web performance and accessibility."},
	{"backend", "后端", "Backend", "Server-side development: APIs, services, application databases, web frameworks, architecture and system design."},
	{"mobile", "移动端", "Mobile", "iOS, Android, mini programs and cross-platform app development."},
	{"ai", "AI 与机器学习", "AI & ML", "Artificial intelligence, machine learning, large language models, and building with or using AI tools."},
	{"data", "数据", "Data", "Data engineering, analytics, statistics, visualization and database internals."},
	{"ops", "运维与云", "Ops & cloud", "Operations and DevOps: servers, networking, containers, Kubernetes, cloud platforms, CI/CD, self-hosting."},
	{"security", "安全", "Security", "Information security, privacy, vulnerabilities, cryptography."},
	{"languages", "编程语言", "Languages", "Programming languages themselves: features, compilers, runtimes, type systems, learning a language."},
	{"tools", "开源与工具", "Open source & tools", "Open-source projects, developer tools, editors, terminals, productivity apps, gadgets and hardware setups."},
	{"design", "设计", "Design", "Visual, interface, interaction, graphic and type design."},
	{"product", "产品与创业", "Product & startups", "Product management, startups, indie making, business, marketing and growth."},
	{"career", "职场", "Career", "Work and careers: jobs, interviews, teams, management, remote work."},
	{"life", "生活随笔", "Life & essays", "Personal life, essays, reflections, diaries, weekly notes and year reviews."},
	{"reading", "读书", "Reading", "Books: reading notes, reviews, reading lists."},
	{"travel", "旅行与摄影", "Travel & photos", "Travel writing and photography."},
}

// TagBySlug finds a tag in the list.
func TagBySlug(slug string) (Tag, bool) {
	for _, t := range Tags {
		if t.Slug == slug {
			return t, true
		}
	}
	return Tag{}, false
}
