// Command screenshots takes the documentation's screenshots of a running
// demo site (scripts/docs_screenshots.sh starts one): the shots of
// shots.json, each page's first screen as WebP, with the site's
// administrator signed in for the admin pages. It drives the Chrome
// installed on the machine (chromedp).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// shot - one screenshot: the page at Path, or the first link on From whose
// address matches Link (and whose text holds Text) - a page whose address
// has an id the demo content gives; Then - a link of that page to follow
// in turn (an item's Edit).
type shot struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	From   string `json:"from"`
	Link   string `json:"link"`
	Text   string `json:"text"`
	Then   string `json:"then"`
	Guest  bool   `json:"guest"`
	Scroll int    `json:"scroll"`
}

func main() {
	base := flag.String("base", "http://127.0.0.1:38600", "the demo site")
	out := flag.String("out", "docs/assets/screens", "where the screenshots go")
	list := flag.String("shots", "shots.json", "the shots")
	width := flag.Int("width", 1200, "the window's width (CSS pixels)")
	height := flag.Int("height", 760, "the window's height")
	scale := flag.Float64("scale", 1.5, "device pixels a CSS pixel")
	flag.Parse()
	if err := run(*base, *out, *list, *width, *height, *scale); err != nil {
		fmt.Fprintln(os.Stderr, "screenshots:", err)
		os.Exit(1)
	}
}

func run(base, out, list string, width, height int, scale float64) error {
	b, err := os.ReadFile(list)
	if err != nil {
		return err
	}
	var shots []shot
	if err := json.Unmarshal(b, &shots); err != nil {
		return fmt.Errorf("%s: %w", list, err)
	}
	login, pass := os.Getenv("SHOT_LOGIN"), os.Getenv("SHOT_PASSWORD")
	if login == "" || pass == "" {
		return errors.New("SHOT_LOGIN and SHOT_PASSWORD: the demo site's administrator")
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.WindowSize(width, height),
		chromedp.Flag("hide-scrollbars", true),
	)
	actx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()
	ctx, cancel := chromedp.NewContext(actx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if err := chromedp.Run(ctx, chromedp.EmulateViewport(int64(width), int64(height), chromedp.EmulateScale(scale))); err != nil {
		return fmt.Errorf("starting Chrome: %w", err)
	}

	signedIn := false
	for _, s := range shots {
		if !s.Guest && !signedIn {
			if err := signIn(ctx, base, login, pass); err != nil {
				return err
			}
			signedIn = true
		}
		path := s.Path
		if path == "" {
			path, err = findLink(ctx, base+s.From, s.Link, s.Text)
			if err == nil && s.Then != "" {
				path, err = findLink(ctx, base+path, s.Then, "")
			}
			if err != nil {
				return fmt.Errorf("%s: %w", s.Name, err)
			}
		}
		file := filepath.Join(out, s.Name+".webp")
		if err := capture(ctx, base+path, s.Scroll, file); err != nil {
			return fmt.Errorf("%s (%s): %w", s.Name, path, err)
		}
		fmt.Printf("%s  %s\n", file, path)
	}
	return nil
}

// signIn signs the administrator in on the site's login form.
func signIn(ctx context.Context, base, login, pass string) error {
	err := chromedp.Run(ctx,
		chromedp.Navigate(base+"/login"),
		chromedp.WaitVisible("#loginIdentity", chromedp.ByID),
		chromedp.SendKeys("#loginIdentity", login, chromedp.ByID),
		chromedp.SendKeys("#loginPassword", pass, chromedp.ByID),
		chromedp.Click(`form.auth-form button[type="submit"]`, chromedp.ByQuery),
		chromedp.WaitNotPresent("#loginPassword", chromedp.ByID),
	)
	if err != nil {
		return fmt.Errorf("signing in: %w", err)
	}
	return nil
}

// findLink - the address of the first link on the page at url matching
// pattern (and holding text, when given).
func findLink(ctx context.Context, url, pattern, text string) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", err
	}
	var links []struct {
		Href string `json:"href"`
		Text string `json:"text"`
	}
	if err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Evaluate(`Array.from(document.querySelectorAll("a[href]")).map(a => ({href: a.getAttribute("href"), text: a.textContent.trim()}))`, &links),
	); err != nil {
		return "", err
	}
	origin := url[:strings.Index(url[len("http://"):], "/")+len("http://")]
	for _, l := range links {
		// A link of the site in full (an item's URL) as a path.
		href := strings.TrimPrefix(l.Href, origin)
		if re.MatchString(href) && (text == "" || strings.Contains(l.Text, text)) {
			return href, nil
		}
	}
	return "", fmt.Errorf("no link like %s on %s", pattern, url)
}

// capture saves the first screen of the page at url (scrolled down by
// scroll CSS pixels) as WebP.
func capture(ctx context.Context, url string, scroll int, file string) error {
	var buf []byte
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Evaluate(`document.fonts.ready.then(() => true)`, nil, func(p *runtime.EvaluateParams) *runtime.EvaluateParams { return p.WithAwaitPromise(true) }),
		chromedp.Evaluate(fmt.Sprintf(`window.scrollTo(0, %d); true`, scroll), nil),
		chromedp.Sleep(700*time.Millisecond),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			buf, err = page.CaptureScreenshot().WithFormat(page.CaptureScreenshotFormatWebp).WithQuality(82).Do(ctx)
			return err
		}),
	)
	if err != nil {
		return err
	}
	return os.WriteFile(file, buf, 0o644)
}
