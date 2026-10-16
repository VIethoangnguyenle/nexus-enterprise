package thumbnail

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// maxOutputBytes bounds a thumbnail a tool writes; a real one is a few tens of
// kilobytes.
const maxOutputBytes = 4 << 20

// DefaultToolID is the uid and gid (nobody) the external tools run as when the
// service itself runs as root.
const DefaultToolID = 65534

// videoFormats are the containers a video thumbnail is drawn from: the
// demuxers (as ffmpeg names them) of MP4/MOV/3GP, Matroska/WebM and AVI. A file
// named .mp4 that is really a playlist, a concat script or any other demuxer is
// refused before ffmpeg opens anything it refers to.
const videoFormats = "mov,mp4,m4a,3gp,3g2,mj2,matroska,webm,avi"

// Exec draws thumbnails: pictures in-process, PDFs with pdftoppm (poppler) and
// videos with ffmpeg.
type Exec struct {
	// euid is the effective uid of this process; the tools are only dropped to
	// toolUID/toolGID when it is 0, since only root may change identity.
	euid             int
	toolUID, toolGID uint32
	// chown hands the work dir and the staged source to the tool user; nil
	// means os.Chown. Tests replace it so staging can be checked without root.
	chown func(name string, uid, gid int) error
}

func (e *Exec) setOwner(name string) error {
	chown := e.chown
	if chown == nil {
		chown = os.Chown
	}
	return chown(name, int(e.toolUID), int(e.toolGID))
}

// NewExec returns the production Renderer. When the service runs as root, the
// tools run as THUMBNAIL_TOOL_UID / THUMBNAIL_TOOL_GID (default 65534); setting
// the uid to 0 keeps them as root. As any other user nothing changes.
func NewExec() *Exec {
	return &Exec{
		euid:    os.Geteuid(),
		toolUID: envID("THUMBNAIL_TOOL_UID"),
		toolGID: envID("THUMBNAIL_TOOL_GID"),
	}
}

func envID(key string) uint32 {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			return uint32(n)
		}
	}
	return DefaultToolID
}

// sysProcAttr is what a tool's process is started with: its own identity, no
// supplementary groups, when this process is root and so may drop privileges;
// nil (unchanged) otherwise.
func sysProcAttr(euid int, uid, gid uint32) *syscall.SysProcAttr {
	if euid != 0 || uid == 0 {
		return nil
	}
	return &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uid, Gid: gid}}
}

func (e *Exec) attr() *syscall.SysProcAttr { return sysProcAttr(e.euid, e.toolUID, e.toolGID) }

// Render implements Renderer.
func (e *Exec) Render(ctx context.Context, kind Kind, srcPath string, size int) ([]byte, error) {
	if size <= 0 {
		return nil, fmt.Errorf("invalid thumbnail size %d", size)
	}
	switch kind {
	case Image:
		return renderImage(ctx, srcPath, size)
	case PDF:
		return e.renderPDF(ctx, srcPath, size)
	case Video:
		return e.renderVideo(ctx, srcPath, size)
	}
	return nil, fmt.Errorf("unsupported thumbnail kind %d", kind)
}

// workDir makes the private directory a tool works in, and the path of the
// source as the tool will see it. When the tool runs as another user the
// directory is theirs and the source is linked (or, across filesystems, copied)
// into it, because the service's own temp directories are private to it.
func (e *Exec) workDir(srcPath string) (dir, src string, err error) {
	dir, err = os.MkdirTemp("", "nexus-thumb-*")
	if err != nil {
		return "", "", fmt.Errorf("temp dir: %w", err)
	}
	if e.attr() == nil {
		return dir, srcPath, nil
	}
	src = filepath.Join(dir, "input")
	if err := e.setOwner(dir); err != nil {
		os.RemoveAll(dir)
		return "", "", fmt.Errorf("work dir owner: %w", err)
	}
	if err := os.Link(srcPath, src); err != nil {
		if err := copyFile(srcPath, src); err != nil {
			os.RemoveAll(dir)
			return "", "", fmt.Errorf("stage source: %w", err)
		}
	}
	if err := e.setOwner(src); err != nil {
		os.RemoveAll(dir)
		return "", "", fmt.Errorf("source owner: %w", err)
	}
	return dir, src, nil
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	return err
}

// renderPDF draws page 1 only, with the longest side of the page at size.
func (e *Exec) renderPDF(ctx context.Context, srcPath string, size int) ([]byte, error) {
	dir, src, err := e.workDir(srcPath)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	prefix := filepath.Join(dir, "page")
	if err := run(ctx, e.attr(), dir, "pdftoppm", "-q", "-f", "1", "-l", "1", "-scale-to", strconv.Itoa(size),
		"-jpeg", "-jpegopt", "quality="+strconv.Itoa(jpegQuality), "-singlefile", src, prefix); err != nil {
		return nil, err
	}
	return readOutput(prefix + ".jpg")
}

// videoArgs is the ffmpeg command line for one frame. The input options come
// before -i: only the local file protocol, so a container that names other
// files or URLs cannot make ffmpeg fetch them, and only the containers above.
func videoArgs(src, out, scale, at string) []string {
	return []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y",
		"-protocol_whitelist", "file", "-format_whitelist", videoFormats,
		"-ss", at, "-i", src, "-frames:v", "1", "-vf", scale, "-q:v", "4", out}
}

// renderVideo takes the frame a second in, or the first one when the video is
// shorter than that, scaled so the longest side is size.
func (e *Exec) renderVideo(ctx context.Context, srcPath string, size int) ([]byte, error) {
	dir, src, err := e.workDir(srcPath)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	out := filepath.Join(dir, "frame.jpg")
	n := strconv.Itoa(size)
	// Escaped commas: inside a filter, an unescaped one starts the next filter.
	scale := `scale=if(gt(iw\,ih)\,min(iw\,` + n + `)\,-2):if(gt(iw\,ih)\,-2\,min(ih\,` + n + `))`

	var lastErr error
	for _, at := range []string{"1", "0"} {
		_ = os.Remove(out)
		lastErr = run(ctx, e.attr(), dir, "ffmpeg", videoArgs(src, out, scale, at)...)
		if lastErr == nil {
			if b, err := readOutput(out); err == nil {
				return b, nil
			}
		}
		if ctx.Err() != nil || errors.Is(lastErr, ErrToolMissing) {
			break
		}
	}
	if lastErr == nil {
		lastErr = ErrNoOutput
	}
	return nil, lastErr
}

func readOutput(path string) ([]byte, error) {
	st, err := os.Stat(path)
	if err != nil || st.Size() == 0 || st.Size() > maxOutputBytes {
		return nil, ErrNoOutput
	}
	return os.ReadFile(path)
}

// run executes name with args, without a shell, in dir, stopping with ctx, with
// the process attributes attr (nil: as this process). A tool that is not
// installed is ErrToolMissing.
func run(ctx context.Context, attr *syscall.SysProcAttr, dir, name string, args ...string) error {
	path, err := exec.LookPath(name)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrToolMissing, name)
	}
	var stderr tail
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = dir
	cmd.Env = toolEnv(attr != nil, dir)
	cmd.SysProcAttr = attr
	cmd.Stderr = &stderr
	// After ctx ends the process is killed; this bounds the wait for its pipes.
	cmd.WaitDelay = 2 * time.Second
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("%s: %w", name, ctx.Err())
		}
		return fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// toolEnv is the environment a tool gets: what it needs to find fonts and its
// cache, and none of the service's secrets. A tool running as another user
// gets the work directory as HOME, since the service's own is not theirs.
func toolEnv(dropped bool, dir string) []string {
	keep := []string{"PATH", "HOME", "LANG", "LC_ALL", "XDG_CACHE_HOME", "XDG_DATA_DIRS", "FONTCONFIG_PATH", "FONTCONFIG_FILE"}
	var env []string
	for _, k := range keep {
		if dropped && (k == "HOME" || k == "XDG_CACHE_HOME") {
			continue
		}
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	if dropped {
		env = append(env, "HOME="+dir, "XDG_CACHE_HOME="+dir)
	}
	return env
}

// tail keeps the last few hundred bytes written to it, enough to say why a tool
// failed without holding its whole output.
type tail struct{ buf bytes.Buffer }

const tailMax = 512

func (t *tail) Write(p []byte) (int, error) {
	t.buf.Write(p)
	if over := t.buf.Len() - tailMax; over > 0 {
		t.buf.Next(over)
	}
	return len(p), nil
}

func (t *tail) String() string { return t.buf.String() }
