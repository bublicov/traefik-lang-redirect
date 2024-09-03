package traefik_lang_redirect

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

const StrategyHeader = "header"
const StrategyPath = "path"
const StrategyQuery = "query"

// Config the plugin configuration.
type Config struct {
	Languages               []string `yaml:"languages"`
	DefaultLanguage         string   `yaml:"defaultLanguage"`
	DefaultLanguageHandling bool     `yaml:"defaultLanguageHandling"`
	LanguageStrategy        string   `yaml:"languageStrategy"`
	LanguageParam           string   `yaml:"languageParam"`
	RedirectAfterHandling   bool     `yaml:"redirectAfterHandling"`
}

// CreateConfig creates the default plugin configuration.
func CreateConfig() *Config {
	return &Config{
		Languages:               []string{},
		DefaultLanguage:         "",
		DefaultLanguageHandling: false,
		LanguageStrategy:        "header",
		LanguageParam:           "lang",
		RedirectAfterHandling:   false,
	}
}

// LangRedirect a plugin.
type LangRedirect struct {
	next   http.Handler
	config *Config
}

// New creates a new plugin.
func New(ctx context.Context, next http.Handler, config *Config, name string) (http.Handler, error) {
	if len(config.Languages) == 0 {
		return nil, fmt.Errorf("languages are required")
	}

	if config.DefaultLanguage == "" {
		return nil, fmt.Errorf("DefaultLanguage is required")
	}

	if config.LanguageStrategy == StrategyQuery && config.LanguageParam == "" {
		return nil, fmt.Errorf("languageParam is required when LanguageStrategy is 'query'")
	}

	return &LangRedirect{
		next:   next,
		config: config,
	}, nil
}

// ServeHTTP implements the http.Handler interface.
func (g *LangRedirect) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	languageByHeader := g.getPreferredLanguage(r.Header.Get("Accept-Language"))

	if languageByHeader != "" && (languageByHeader != g.config.DefaultLanguage || g.config.DefaultLanguageHandling) {
		if strategy, err := g.getStrategy(); err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		} else {
			// Maybe lang already exist
			languageByRequest := strategy.GetLanguage(r)
			// Set lang
			if languageByRequest == "" || languageByRequest != languageByHeader {
				// Executing
				strategy.SetLanguage(w, r, languageByHeader)
				// Stop further execution if a redirect perform
				if g.config.RedirectAfterHandling {
					http.Redirect(w, r, r.URL.String(), http.StatusFound)
					return
				}
			}
		}
	}

	g.next.ServeHTTP(w, r)
}

/* Helpers
 * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * */

func (g *LangRedirect) getPreferredLanguage(acceptLanguage string) string {
	languages := parseAcceptLanguage(acceptLanguage)
	for _, lang := range languages {
		for _, supportedLang := range g.config.Languages {
			if lang == supportedLang {
				return lang
			}
		}
	}
	return g.config.DefaultLanguage
}

func parseAcceptLanguage(acceptLanguage string) []string {
	parts := strings.Split(acceptLanguage, ",")
	languages := make([]string, 0, len(parts))
	for _, part := range parts {
		lang := strings.SplitN(part, ";", 2)[0]
		lang = strings.TrimSpace(lang)
		languages = append(languages, lang)
	}
	return languages
}

func (g *LangRedirect) getStrategy() (Strategy, error) {
	switch g.config.LanguageStrategy {
	case StrategyHeader:
		return &HeaderStrategy{}, nil
	case StrategyPath:
		return &PathStrategy{}, nil
	case StrategyQuery:
		return &QueryStrategy{languageParam: g.config.LanguageParam}, nil
	default:
		return nil, fmt.Errorf("invalid LanguageStrategy: %s", g.config.LanguageStrategy)
	}
}

/* Handlers
 * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * */

type Strategy interface {
	GetLanguage(r *http.Request) string
	SetLanguage(w http.ResponseWriter, r *http.Request, language string)
}

type HeaderStrategy struct {
}

type PathStrategy struct {
}

type QueryStrategy struct {
	languageParam string
}

func (h *HeaderStrategy) GetLanguage(r *http.Request) string {
	return r.Header.Get("Accept-Language")
}

func (h *HeaderStrategy) SetLanguage(w http.ResponseWriter, r *http.Request, language string) {
	r.Header.Set("Accept-Language", language)
}

func (p *PathStrategy) GetLanguage(r *http.Request) string {
	segments := strings.Split(r.URL.Path, "/")
	if len(segments) > 1 && len(segments[1]) == 2 {
		return segments[1]
	}
	return ""
}

func (p *PathStrategy) SetLanguage(w http.ResponseWriter, r *http.Request, language string) {
	if r.URL.Path == "/" {
		r.URL.Path = "/" + language
	} else {
		r.URL.Path = "/" + language + r.URL.Path
	}
}

func (q *QueryStrategy) GetLanguage(r *http.Request) string {
	query := r.URL.Query()
	return query.Get(q.languageParam)
}

func (q *QueryStrategy) SetLanguage(w http.ResponseWriter, r *http.Request, language string) {
	query := r.URL.Query()
	query.Set(q.languageParam, language)
	r.URL.RawQuery = query.Encode()
}
