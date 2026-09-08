"""Run a project's preview generator independently of the editor window."""
import contextlib
import hashlib
import json
import os
from pathlib import Path
import runpy
import shutil
import subprocess
import sys
import tempfile
import time
import traceback


def write_json(path, value):
    temp = path.with_name(path.name + "." + str(os.getpid()) + ".tmp")
    temp.write_text(json.dumps(value), encoding="utf-8")
    os.replace(temp, path)


def read_json(path, default):
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError:
        return default


def cached_json(path):
    try:
        value = read_json(path, {})
        return value if isinstance(value, dict) else {}
    except (OSError, ValueError):
        return {}


class IncrementalPreviews:
    """Reuse map renders while the project generator rebuilds current geometry."""

    def __init__(self, settings, script, environment, output, folder, force):
        self.settings = settings
        self.script = script
        self.environment = environment
        self.output = output
        self.cache_path = folder / "render-cache.json"
        self.previous = cached_json(self.cache_path)
        self.force = force
        self.context = None
        self.images = {}
        self.sources = {}
        self.rendered = 0
        self.reused = 0
        self.legacy = {}
        manifest = cached_json(output / "manifest.json")
        for group in ("hulls", "modules"):
            entries = manifest.get(group, {})
            if isinstance(entries, dict):
                for entry in entries.values():
                    self.add_legacy(entry)

    def add_legacy(self, entry):
        if not isinstance(entry, dict):
            return
        if isinstance(entry.get("png"), str):
            self.legacy[entry["png"]] = entry.get("src_md5")
        themes = entry.get("themes", {})
        if isinstance(themes, dict):
            for variant in themes.values():
                self.add_legacy(variant)

    @staticmethod
    def signature(path):
        data = path.read_bytes()
        info = path.stat()
        return hashlib.md5(data).hexdigest(), info.st_size, info.st_mtime_ns

    def source(self, path):
        path = Path(path).resolve()
        signature = self.signature(path)
        before = self.sources.setdefault(path, signature)
        if before != signature:
            raise RuntimeError(f"Map changed during preview generation: {path}. The queued save will retry it.")
        return path, signature[0]

    def validate_sources(self):
        for path in self.sources:
            self.source(path)

    def install(self):
        original_render = self.settings.get("render")
        if not callable(original_render):
            return False
        self.original_render = original_render
        self.settings["render"] = self.render
        original_dmm = self.settings.get("Dmm")
        if callable(original_dmm):
            def load_map(path, *args, **kwargs):
                self.source(path)
                result = original_dmm(path, *args, **kwargs)
                self.source(path)
                return result
            self.settings["Dmm"] = load_map
        return True

    def render(self, tool, source, destination, temp, dmm):
        source, digest = self.source(source)
        if self.context is None:
            from PIL import Image
            self.context = {
                "version": 1,
                "generator": hashlib.sha256(self.script.read_bytes()).hexdigest(),
                "renderer": hashlib.sha256(Path(tool).read_bytes()).hexdigest(),
                "python": sys.version,
                "pillow": Image.__version__,
                "environment": os.path.normcase(str(self.environment.resolve())),
            }
        name = destination.name
        target = self.output / name
        record = {"source": os.path.normcase(str(source)), "src_md5": digest}
        previous_images = self.previous.get("images", {})
        previous = previous_images.get(name) if isinstance(previous_images, dict) else None
        matching = (not self.force and self.previous.get("context") == self.context
                    and isinstance(previous, dict)
                    and all(previous.get(key) == value for key, value in record.items()))
        # Existing committed previews already carry source hashes. Adopt those
        # on the first incremental run instead of needlessly rendering a fleet.
        legacy = (not self.force and not self.previous
                  and self.legacy.get(name) == digest)
        reused = False
        if (matching or legacy) and target.is_file():
            image_hash = hashlib.sha256(target.read_bytes()).hexdigest()
            if matching:
                reused = previous.get("png_sha256") == image_hash
            elif legacy:
                from PIL import Image
                try:
                    with Image.open(target) as image:
                        image.verify()
                    reused = True
                except (OSError, ValueError, SyntaxError):
                    pass
            if reused:
                shutil.copy2(target, destination)
        if reused:
            self.reused += 1
        else:
            print(f"Rendering changed or missing preview: {name}", flush=True)
            self.original_render(tool, source, destination, temp, dmm)
            self.source(source)
            self.rendered += 1
        record["png_sha256"] = hashlib.sha256(destination.read_bytes()).hexdigest()
        self.images[name] = record

    def save(self):
        try:
            write_json(self.cache_path, {"context": self.context, "images": self.images})
        except OSError as error:
            # Published previews remain usable if the optional cache cannot be saved.
            print(f"Could not save the preview cache: {error}", flush=True)


@contextlib.contextmanager
def lock(path, blocking=True):
    # Kernel-owned locks are released even if Python or the editor crashes.
    with path.open("a+b") as file:
        file.seek(0)
        if os.name == "nt":
            import msvcrt
            if path.stat().st_size == 0:
                file.write(b"\0")
                file.flush()
            while True:
                try:
                    file.seek(0)
                    msvcrt.locking(file.fileno(), msvcrt.LK_NBLCK, 1)
                    break
                except OSError:
                    if not blocking:
                        yield False
                        return
                    time.sleep(.05)
            try:
                yield True
            finally:
                file.seek(0)
                msvcrt.locking(file.fileno(), msvcrt.LK_UNLCK, 1)
        else:
            import fcntl
            try:
                fcntl.flock(file, fcntl.LOCK_EX | (0 if blocking else fcntl.LOCK_NB))
            except BlockingIOError:
                yield False
                return
            try:
                yield True
            finally:
                fcntl.flock(file, fcntl.LOCK_UN)


def generate(root, environment, folder, force=False):
    script = root / "tools/ship_previews/generate_ship_previews.py"
    # Run the project's maintained implementation, including its smoothing fixes.
    namespace = runpy.run_path(str(script))
    settings = namespace["main"].__globals__
    output = root / "voidcrew/modules/ship_upgrades/previews"
    output.parent.mkdir(parents=True, exist_ok=True)
    cache = IncrementalPreviews(settings, script, environment, output, folder, force)
    incremental = cache.install()
    if not os.environ.get("DMM_TOOLS"):
        tool = shutil.which("dmm-tools")
        if tool:
            os.environ["DMM_TOOLS"] = tool
    with tempfile.TemporaryDirectory(prefix=".strongdmm-previews-", dir=output.parent) as temp:
        stage = Path(temp)
        settings["OUTPUT_DIR"] = stage
        settings["ENVIRONMENT"] = str(environment)
        namespace["main"]()
        cache.validate_sources()
        manifest = read_json(stage / "manifest.json", None)
        if not isinstance(manifest, dict) or not isinstance(manifest.get("hulls"), dict) or not isinstance(manifest.get("modules"), dict):
            raise RuntimeError("The generator did not produce a valid preview manifest")
        files = sorted(stage.glob("*.png")) + [stage / "manifest.json"]
        output.mkdir(parents=True, exist_ok=True)
        backup = stage / "backup"
        backup.mkdir()
        replaced = []
        try:
            for source in files:
                target = output / source.name
                existed = target.exists()
                if existed and target.read_bytes() == source.read_bytes():
                    continue
                if existed:
                    shutil.copy2(target, backup / source.name)
                os.replace(source, target)
                replaced.append((target, existed))
        except BaseException:
            for target, existed in reversed(replaced):
                if existed:
                    os.replace(backup / target.name, target)
                else:
                    target.unlink(missing_ok=True)
            raise
    if incremental:
        cache.save()
    print("Purchase previews updated.", flush=True)
    if incremental:
        print(f"Preview images: {cache.rendered} rendered, {cache.reused} reused.", flush=True)


def request(folder, root, environment, force=False):
    queue = folder / "request.json"
    state = folder / "status.json"
    # Queue changes and worker shutdown share a short lock, so a save arriving
    # as the current worker exits cannot be stranded. Multiple editors share it.
    guard = lock(folder / "queue.lock")
    guard.__enter__()
    worker = None
    try:
        previous = read_json(queue, {"revision": 0})
        revision = previous["revision"] + 1
        write_json(queue, {"revision": revision, "root": str(root), "environment": str(environment),
                           "force": force or previous.get("force", False)})
        worker = lock(folder / "worker.lock", blocking=False)
        if not worker.__enter__():
            worker.__exit__(None, None, None)
            worker = None
            status = read_json(state, {})
            status["queued"] = True
            status["updated"] = time.time()
            write_json(state, status)
            return
        while True:
            item = read_json(queue, {})
            revision = item["revision"]
            # A full rebuild belongs to this pass. Later saves queue an ordinary
            # incremental pass unless another full rebuild is requested.
            write_json(queue, {**item, "force": False})
            write_json(state, {"phase": "running", "queued": False, "pid": os.getpid(), "updated": time.time()})
            guard.__exit__(None, None, None)
            guard = None
            log_path = folder / "generation.log"
            with log_path.open("w", encoding="utf-8") as log:
                flags = subprocess.CREATE_NO_WINDOW if os.name == "nt" else 0
                result = subprocess.run(
                    [sys.executable, "-B", "-u", __file__, "--run", str(folder), item["root"], item["environment"]]
                    + (["--force"] if item.get("force") else []),
                    cwd=item["root"], stdout=log, stderr=log, creationflags=flags,
                )
            guard = lock(folder / "queue.lock")
            guard.__enter__()
            latest = read_json(queue, {})
            if result.returncode != 0 and item.get("force"):
                latest["force"] = True
                write_json(queue, latest)
            if latest["revision"] != revision:
                continue
            write_json(state, {"phase": "complete" if result.returncode == 0 else "failed",
                              "queued": False, "updated": time.time()})
            return
    finally:
        # Release the worker while still holding the queue lock.
        if worker is not None:
            worker.__exit__(None, None, None)
        if guard is not None:
            guard.__exit__(None, None, None)


if __name__ == "__main__":
    if sys.argv[1] == "--run":
        generate(Path(sys.argv[3]), Path(sys.argv[4]), Path(sys.argv[2]), "--force" in sys.argv[5:])
    else:
        folder = Path(sys.argv[1])
        try:
            request(folder, Path(sys.argv[2]), Path(sys.argv[3]), "--force" in sys.argv[4:])
        except BaseException:
            with (folder / "generation.log").open("a", encoding="utf-8") as log:
                traceback.print_exc(file=log)
            write_json(folder / "status.json", {"phase": "failed", "queued": False, "updated": time.time()})
            raise
