package check

import (
	"github.com/kite-plus/explore/internal/i18n"
	"github.com/kite-plus/explore/internal/model"
)

// hint is one problem's advice in both languages.
type hint struct{ en, zh string }

// hints holds the advice for each problem, with an entry under the empty
// generator as the fallback. It is the same table as
// docs/design/worker.md section 6.2.
var hints = map[model.ProblemCode]map[model.Generator]hint{
	model.ProblemFeedNotFound: {
		"": {
			"No RSS, Atom or JSON Feed was found on this site. If the blog has one, submit its address as the feed URL.",
			"在这个网站上没有找到 RSS、Atom 或 JSON Feed。如果博客有订阅源，提交时直接填写订阅地址。",
		},
		model.GeneratorHexo: {
			"Hexo needs a plugin for feeds: install hexo-generator-feed, then deploy again.",
			"Hexo 需要插件才能输出订阅源：安装 hexo-generator-feed 后重新部署。",
		},
		model.GeneratorHalo: {
			"Make sure the feed plugin (plugin-feed) is enabled in the Halo console.",
			"请在 Halo 后台确认订阅插件（plugin-feed）已启用。",
		},
		model.GeneratorHugo: {
			"Hugo writes /index.xml by default; check that RSS output is not turned off in the configuration or theme.",
			"Hugo 默认输出 /index.xml，请检查配置或主题是否关闭了 RSS 输出。",
		},
		model.GeneratorWordPress: {
			"WordPress serves /feed/ by default; check that no plugin disables feeds.",
			"WordPress 默认提供 /feed/，请检查是否有插件关闭了订阅源。",
		},
	},
	model.ProblemRobotsDisallowed: {
		"": {
			"The site's robots.txt does not allow KiteExplore to read the feed. Allow the feed address for User-agent: KiteExplore.",
			"网站的 robots.txt 不允许 KiteExplore 读取订阅源。请为 User-agent: KiteExplore 放行订阅地址。",
		},
	},
	model.ProblemHTTPError: {
		"": {
			"The server did not return the page. Check that the address is correct and publicly reachable.",
			"服务器没有正常返回页面。请确认地址正确，并且可以从公网访问。",
		},
	},
	model.ProblemTooLarge: {
		"": {
			"The feed is larger than 5 MiB. Limit how many posts it contains.",
			"订阅源超过了 5 MiB，请减少其中的文章数量。",
		},
		model.GeneratorHugo: {
			"Hugo puts every page in the feed by default. Set services.rss.limit, for example to 20.",
			"Hugo 默认把全部页面写进订阅源。请设置 services.rss.limit，例如 20。",
		},
	},
	model.ProblemParseError: {
		"": {
			"The address does not return a valid RSS, Atom or JSON Feed document.",
			"这个地址返回的不是合法的 RSS、Atom 或 JSON Feed。",
		},
	},
	model.ProblemLinksOffDomain: {
		"": {
			"Most post links point to another domain. Check the site address your blog system is configured with.",
			"大部分文章链接指向了其他域名。请检查博客系统里配置的站点地址。",
		},
		model.GeneratorHexo: {
			"Set url in _config.yml to your blog's address; the default http://example.com sends every link elsewhere.",
			"请把 _config.yml 里的 url 改成博客的地址；默认的 http://example.com 会让所有链接指向别处。",
		},
		model.GeneratorHugo: {
			"Set baseURL in the Hugo configuration to your blog's address.",
			"请把 Hugo 配置里的 baseURL 改成博客的地址。",
		},
		model.GeneratorHalo: {
			"Set halo.external-url to your blog's public address.",
			"请把 halo.external-url 改成博客的公网地址。",
		},
		model.GeneratorWordPress: {
			"Check Settings → General → Site Address (URL).",
			"请检查 设置 → 常规 → 站点地址（URL）。",
		},
		model.GeneratorJekyll: {
			"Set url in _config.yml to your blog's address.",
			"请把 _config.yml 里的 url 改成博客的地址。",
		},
	},
	model.ProblemLinksElsewhere: {
		"": {
			"The posts in this feed link to other sites. Explore only lists posts published on the blog itself, so readers always land on the author's site.",
			"这个订阅源里的文章都链接到其他网站。Explore 只收录发布在博客本身的文章，保证读者点开后到达的是作者的网站。",
		},
	},
	model.ProblemSomeLinksOffDomain: {
		"": {
			"A few post links point to another domain and will be skipped. If the blog really spans several domains, say so when submitting.",
			"少数文章链接指向了其他域名，这些文章会被跳过。如果博客确实跨多个域名，提交时说明。",
		},
	},
	model.ProblemNoValidItems: {
		"": {
			"No post in the feed could be used; the other problems explain why.",
			"订阅源里没有一篇可用的文章，原因见其他问题。",
		},
	},
	model.ProblemStale: {
		"": {
			"The feed has no post from the last 12 months.",
			"订阅源里没有近 12 个月发布的文章。",
		},
	},
	model.ProblemNoDates: {
		"": {
			"Some posts have no publish date, so they will not appear on the home page.",
			"有文章没有发布时间，它们不会出现在首页。",
		},
		model.GeneratorHugo: {
			"Some posts have no publish date. Add date to their front matter.",
			"有文章没有发布时间。请在它们的 front matter 里写上 date。",
		},
	},
	model.ProblemDatesUntrusted: {
		"": {
			"Many posts share the same publish time, so their dates cannot be trusted. Give every post its own date.",
			"大量文章的发布时间相同，这些日期不可信。请给每篇文章写上各自的日期。",
		},
		model.GeneratorHexo: {
			"Posts without date in their front matter get the file time, which CI resets on every build. Add date to each post.",
			"没写 date 的文章会用文件时间作发布时间，而 CI 每次构建都会重置它。请给每篇文章写上 date。",
		},
	},
	model.ProblemIncludesNonPosts: {
		"": {
			"The feed also lists pages that are not posts. Use a feed that contains only posts.",
			"订阅源里混有不是文章的页面，请改用只含文章的订阅源。",
		},
		model.GeneratorHugo: {
			"The home feed also lists pages such as About. Use the posts section's feed instead.",
			"首页订阅源里混有 About 这类页面，请改用文章分区的订阅源。",
		},
	},
	model.ProblemNoConditionalGet: {
		"": {
			"The server does not support conditional requests (ETag or Last-Modified), so every check downloads the whole feed.",
			"服务器不支持条件请求（ETag 或 Last-Modified），每次检查都要下载完整的订阅源。",
		},
	},
	model.ProblemRedirected: {
		"": {
			"The feed address permanently redirects to a new one. Use the new address.",
			"订阅地址已经永久重定向到新地址，请改用新地址。",
		},
	},
}

// Hint returns the advice for a problem in the given language.
func Hint(code model.ProblemCode, gen model.Generator, lang i18n.Lang) string {
	byGen := hints[code]
	h, ok := byGen[gen]
	if !ok {
		h = byGen[""]
	}
	if lang == i18n.Chinese {
		return h.zh
	}
	return h.en
}

// Localize fills in every problem's hint. Stored reports carry none, so the
// same submission reads correctly in either language.
func Localize(r *model.CheckReport, lang i18n.Lang) {
	for i := range r.Problems {
		r.Problems[i].Hint = Hint(r.Problems[i].Code, r.Generator, lang)
	}
}
