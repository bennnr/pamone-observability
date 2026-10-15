"""Small CLI: install the auto-start hook so apps need no code changes."""
import os
import site
import sys

PTH_NAME = "zz_pamone_obs.pth"
PTH_LINE = "import pamone_obs; pamone_obs.autostart()\n"


def _site_dirs():
    dirs = []
    try:
        dirs.extend(site.getsitepackages())
    except Exception:
        pass
    try:
        dirs.append(site.getusersitepackages())
    except Exception:
        pass
    # de-dup, keep order
    seen = set()
    out = []
    for d in dirs:
        if d and d not in seen:
            seen.add(d)
            out.append(d)
    return out


def install_pth():
    written = []
    for directory in _site_dirs():
        try:
            os.makedirs(directory, exist_ok=True)
            with open(os.path.join(directory, PTH_NAME), "w") as fh:
                fh.write(PTH_LINE)
            written.append(directory)
        except OSError:
            continue
    return written


def main(argv=None):
    args = list(sys.argv[1:] if argv is None else argv)
    cmd = args[0] if args else "help"
    if cmd == "install-pth":
        dirs = install_pth()
        if dirs:
            for directory in dirs:
                print(f"pamone-obs: installed {PTH_NAME} in {directory}")
            return 0
        print("pamone-obs: could not write .pth (no writable site-packages)")
        return 1
    if cmd == "uninstall-pth":
        removed = []
        for directory in _site_dirs():
            path = os.path.join(directory, PTH_NAME)
            try:
                os.remove(path)
                removed.append(path)
            except OSError:
                continue
        for path in removed:
            print(f"pamone-obs: removed {path}")
        return 0 if removed else 1
    print("usage: pamone-obs [install-pth|uninstall-pth]")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
