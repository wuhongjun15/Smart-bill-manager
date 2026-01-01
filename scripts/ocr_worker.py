#!/usr/bin/env python3
"""
Persistent OCR worker for Smart Bill Manager.

Protocol: JSON lines (stdin -> request, stdout -> response).
Request:
  {"id": "...", "type": "ocr", "image_path": "/path/to/img.png", "profile": "default"|"pdf", "debug": false}
Response (always one line):
  {"id": "...", "success": true|false, ...same shape as ocr_cli.py...}

Goal: keep RapidOCR models loaded once to avoid per-request Python startup overhead.
"""

from __future__ import annotations

import contextlib
import json
import os
import sys
import tempfile
import traceback
from importlib import metadata
from pathlib import Path


@contextlib.contextmanager
def suppress_child_output(enabled: bool):
    if not enabled:
        yield
        return

    devnull = open(os.devnull, "w")
    old_stdout_fd = os.dup(1)
    old_stderr_fd = os.dup(2)
    try:
        os.dup2(devnull.fileno(), 1)
        os.dup2(devnull.fileno(), 2)
        yield
    finally:
        try:
            os.dup2(old_stdout_fd, 1)
            os.dup2(old_stderr_fd, 2)
        finally:
            os.close(old_stdout_fd)
            os.close(old_stderr_fd)
            devnull.close()


def truthy(v: str | None) -> bool:
    if v is None:
        return False
    return str(v).strip().lower() in ("1", "true", "yes", "y", "on")


def safe_int(v: str | None, default: int) -> int:
    try:
        return int(str(v).strip())
    except Exception:
        return default


def score_lines(lines: list[dict]) -> float:
    if not lines:
        return 0.0

    total_conf = 0.0
    total_len = 0
    cjk = 0
    digits = 0
    for l in lines:
        t = l.get("text") or ""
        total_len += len(t)
        total_conf += float(l.get("confidence") or 0.0)
        for ch in t:
            o = ord(ch)
            if 0x4E00 <= o <= 0x9FFF:
                cjk += 1
            elif "0" <= ch <= "9":
                digits += 1

    avg_conf = total_conf / max(len(lines), 1)
    content = (cjk * 2.0) + (digits * 1.2) + (max(total_len - cjk - digits, 0) * 0.25)
    return (content + (len(lines) * 10.0)) * (0.25 + avg_conf)


def load_pil_image(path: str):
    try:
        from PIL import Image

        im = Image.open(path)
        im.load()
        return im
    except Exception:
        return None


def reorder_lines_by_box(lines: list[dict]) -> list[dict]:
    if not lines:
        return lines

    processed = []
    for line in lines:
        box = line.get("box") or []
        if not box:
            continue
        try:
            xs = [p[0] for p in box if isinstance(p, (list, tuple)) and len(p) >= 2]
            ys = [p[1] for p in box if isinstance(p, (list, tuple)) and len(p) >= 2]
            if not xs or not ys:
                continue
            minx, maxx = min(xs), max(xs)
            miny, maxy = min(ys), max(ys)
            processed.append(
                {
                    "line": line,
                    "minx": minx,
                    "maxx": maxx,
                    "miny": miny,
                    "maxy": maxy,
                    "height": max(1.0, maxy - miny),
                }
            )
        except Exception:
            continue

    if not processed:
        return lines

    heights = sorted(p["height"] for p in processed)
    mid = len(heights) // 2
    median_h = heights[mid] if len(heights) % 2 else (heights[mid - 1] + heights[mid]) / 2
    row_tol = max(5.0, median_h * 0.6)

    processed.sort(key=lambda p: (p["miny"], p["minx"]))

    rows: list[list[dict]] = []
    current_row: list[dict] = []
    current_maxy = None
    for p in processed:
        if current_row and current_maxy is not None and p["miny"] > current_maxy + row_tol:
            rows.append(current_row)
            current_row = [p]
            current_maxy = p["maxy"]
        else:
            current_row.append(p)
            current_maxy = p["maxy"] if current_maxy is None else max(current_maxy, p["maxy"])
    if current_row:
        rows.append(current_row)

    ordered = []
    for row in rows:
        row.sort(key=lambda p: p["minx"])
        ordered.extend(row)

    return [p["line"] for p in ordered]


def apply_label_value_layout(lines: list[dict], profile: str) -> list[dict]:
    if profile != "default" or not lines:
        return lines

    label_set = {
        "交易单号",
        "交易号",
        "商户单号",
        "订单号",
        "流水号",
        "转账单号",
        "支付时间",
        "付款时间",
        "创建时间",
        "交易时间",
        "转账时间",
        "商户全称",
        "商家",
        "收款方",
        "收款户",
        "收款账户",
        "付款方",
        "收单机构",
        "清算机构",
        "支付方式",
        "付款方式",
        "当前状态",
        "商品",
        "商品说明",
    }

    def norm(t: str) -> str:
        return str(t or "").strip()

    def is_label(t: str) -> bool:
        return norm(t) in label_set

    def box_of(line: dict):
        box = line.get("box") or []
        if not isinstance(box, (list, tuple)) or not box:
            return None
        try:
            xs = [p[0] for p in box if isinstance(p, (list, tuple)) and len(p) >= 2]
            ys = [p[1] for p in box if isinstance(p, (list, tuple)) and len(p) >= 2]
            if not xs or not ys:
                return None
            left = float(min(xs))
            right = float(max(xs))
            top = float(min(ys))
            bottom = float(max(ys))
            return {
                "left": left,
                "right": right,
                "top": top,
                "bottom": bottom,
                "cx": (left + right) / 2.0,
                "cy": (top + bottom) / 2.0,
            }
        except Exception:
            return None

    idx_by_line = {id(l): i for i, l in enumerate(lines)}
    geo = [(l, box_of(l)) for l in lines]

    used = set()
    out = list(lines)

    for label_line, label_box in geo:
        if label_box is None:
            continue
        if not is_label(label_line.get("text") or ""):
            continue
        label_idx = idx_by_line.get(id(label_line))
        if label_idx is None or label_idx in used:
            continue

        best = None
        best_score = 1e18
        for value_line, value_box in geo:
            if value_box is None:
                continue
            value_idx = idx_by_line.get(id(value_line))
            if value_idx is None or value_idx == label_idx or value_idx in used:
                continue

            # Value should be on the right side of the label, roughly aligned by row.
            if value_box["left"] < label_box["right"] - 5:
                continue
            dy = abs(value_box["cy"] - label_box["cy"])
            dx = value_box["left"] - label_box["right"]
            if dy > 22:
                continue
            score = (dy * 10.0) + dx
            if score < best_score:
                best_score = score
                best = value_idx

        if best is not None:
            val = norm(out[best].get("text") or "")
            if val:
                out[label_idx] = {
                    **out[label_idx],
                    "text": f"{norm(out[label_idx].get('text') or '')}：{val}",
                }
                used.add(best)

    if not used:
        return lines

    return [l for i, l in enumerate(out) if i not in used]


def build_variants(pil_img, profile: str, multipass: int, rotate180: bool):
    if pil_img is None:
        return []
    try:
        from PIL import ImageEnhance, ImageOps
    except Exception:
        return []

    variants = []
    base = pil_img.convert("RGB")

    if profile == "pdf":
        w, h = base.size
        scale = 2

        up = base.resize((w * scale, h * scale))
        up = ImageOps.autocontrast(up)
        up = ImageEnhance.Contrast(up).enhance(1.15)
        up = ImageEnhance.Sharpness(up).enhance(1.6)
        variants.append(("enhance2x", up))

        if multipass >= 2:
            gray = base.convert("L")
            gray = ImageOps.autocontrast(gray)
            gray = gray.resize((w * scale, h * scale))
            variants.append(("gray2x", gray.convert("RGB")))

        if rotate180:
            variants.append(("enhance2x_rot180", up.rotate(180, expand=True)))

    return variants


def configure_model_dir(InferSession):
    model_data_dir = (os.getenv("SBM_OCR_DATA_DIR") or os.getenv("SBM_DATA_DIR") or "").strip()
    if not model_data_dir:
        return None
    try:
        base = Path(model_data_dir).expanduser()
        if base.name != "rapidocr-models":
            base = base / "rapidocr-models"
        if not base.is_absolute():
            base = (Path.cwd() / base).resolve()
        base.mkdir(parents=True, exist_ok=True)
        InferSession.DEFAULT_MODEL_PATH = base
        return str(base)
    except Exception:
        return None


class RapidOCRWorker:
    def __init__(self):
        self._ok = False
        self._error = ""
        self._rapidocr_version = "unknown"
        self._model_dir = None
        self._ocr_by_profile = {}
        self._stderr_suppress = not truthy(os.getenv("SBM_OCR_WORKER_DEBUG"))

        try:
            with suppress_child_output(self._stderr_suppress):
                from rapidocr import RapidOCR
                from rapidocr.inference_engine.base import InferSession

            try:
                self._rapidocr_version = metadata.version("rapidocr")
            except metadata.PackageNotFoundError:
                pass

            self._model_dir = configure_model_dir(InferSession)

            self._RapidOCR = RapidOCR
            self._InferSession = InferSession
            self._ok = True
        except Exception as e:
            self._ok = False
            self._error = str(e)

        warmup = truthy(os.getenv("SBM_OCR_WORKER_WARMUP")) or os.getenv("SBM_OCR_WORKER_WARMUP") is None
        if self._ok and warmup:
            # Best-effort warmup to reduce first-request jitter.
            try:
                self._get_ocr("default")
                self._get_ocr("pdf")
            except Exception:
                # keep worker alive even if warmup fails
                pass

    def _profile_params(self, profile: str) -> dict:
        if profile == "pdf":
            return {
                "Global.max_side_len": 4096,
                "Global.min_height": 10,
                "Global.text_score": 0.35,
            }
        return {}

    def _get_ocr(self, profile: str):
        profile = (profile or "default").strip().lower()
        if profile not in ("default", "pdf"):
            profile = "default"
        if profile in self._ocr_by_profile:
            return self._ocr_by_profile[profile]

        params = self._profile_params(profile)
        with suppress_child_output(self._stderr_suppress):
            ocr = self._RapidOCR(params=params or None)
        self._ocr_by_profile[profile] = ocr
        return ocr

    def _run_ocr_once(self, image_path: str, profile: str):
        ocr = self._get_ocr(profile)
        backend_errors = []
        backend = "rapidocr"
        param_summary = {
            "rapidocr": self._rapidocr_version,
        }
        if self._model_dir:
            param_summary["model_dir"] = self._model_dir

        try:
            with suppress_child_output(self._stderr_suppress):
                out = ocr(image_path)
            return out, backend, backend_errors, param_summary
        except Exception as e:
            backend_errors.append(f"run_failed: {e}")
            # Recreate session once (covers partially downloaded/corrupted models).
            try:
                self._ocr_by_profile.pop(profile, None)
                ocr = self._get_ocr(profile)
                with suppress_child_output(self._stderr_suppress):
                    out = ocr(image_path)
                backend_errors.append("recreated_session_ok")
                return out, backend, backend_errors, param_summary
            except Exception as e2:
                backend_errors.append(f"recreate_failed: {e2}")
                raise RuntimeError("; ".join(backend_errors)) from e2

    def _build_result(self, out, profile: str, backend: str, backend_errors: list[str], param_summary: dict):
        txts = getattr(out, "txts", None) or ()
        scores = getattr(out, "scores", None) or ()
        boxes = getattr(out, "boxes", None)
        if boxes is not None and hasattr(boxes, "tolist"):
            boxes = boxes.tolist()

        lines = []
        full_text_parts = []
        for idx, text in enumerate(txts):
            confidence = 0.0
            if idx < len(scores) and scores[idx] is not None:
                confidence = float(scores[idx])
            line = {"text": text, "confidence": confidence}
            if boxes is not None and idx < len(boxes):
                line["box"] = boxes[idx]
            lines.append(line)

        ordered = reorder_lines_by_box(lines)
        ordered = apply_label_value_layout(ordered, profile)
        for ln in ordered:
            full_text_parts.append(ln.get("text") or "")

        return {
            "text": "\n".join(full_text_parts),
            "lines": ordered,
            "line_count": len(ordered),
            "score": score_lines(ordered),
            "backend": backend,
            "backend_errors": backend_errors,
            "params": param_summary,
        }

    def recognize(self, image_path: str, profile: str, debug: bool):
        if not self._ok:
            return {"success": False, "error": f"RapidOCR worker not available: {self._error}"}

        if not image_path or not os.path.exists(image_path):
            return {"success": False, "error": f"Image file not found: {image_path}"}

        profile = (profile or "default").strip().lower()
        if profile not in ("default", "pdf"):
            profile = "default"

        multipass = safe_int(os.getenv("SBM_RAPIDOCR_MULTIPASS"), 1 if profile == "pdf" else 0)
        rotate180 = truthy(os.getenv("SBM_RAPIDOCR_ROTATE180")) or (profile == "pdf")

        out, backend, backend_errors, param_summary = self._run_ocr_once(image_path, profile)
        best = self._build_result(out, profile, backend, backend_errors, param_summary)
        best_variant = "original"
        variants_debug = []
        if debug:
            variants_debug.append(
                {"variant": best_variant, "score": best["score"], "lines": best["line_count"], "chars": len(best["text"])}
            )

        pil_img = load_pil_image(image_path) if multipass > 0 else None
        variants = build_variants(pil_img, profile, multipass, rotate180)
        if variants:
            with tempfile.TemporaryDirectory(prefix="sbm-ocr-") as td:
                for name, im in variants:
                    out_path = str(Path(td) / f"{name}.png")
                    try:
                        im.save(out_path, format="PNG", optimize=True)
                    except Exception:
                        continue

                    out, backend, backend_errors, param_summary = self._run_ocr_once(out_path, profile)
                    r = self._build_result(out, profile, backend, backend_errors, param_summary)
                    if debug:
                        variants_debug.append(
                            {
                                "variant": name,
                                "score": r["score"],
                                "lines": r["line_count"],
                                "chars": len(r["text"]),
                                "backend": r.get("backend", ""),
                                "backend_errors": r.get("backend_errors", []),
                            }
                        )
                    if r["score"] > best["score"]:
                        best = r
                        best_variant = name

        payload = {
            "success": True,
            "text": best["text"],
            "lines": best["lines"],
            "line_count": best["line_count"],
            "engine": f"rapidocr-{self._rapidocr_version}",
            "profile": profile,
            "variant": best_variant,
            "backend": best.get("backend", ""),
            "backend_errors": best.get("backend_errors", []),
            "params": best.get("params", {}),
        }
        if debug:
            payload["variants"] = variants_debug
        return payload


def write_line(obj: dict):
    sys.stdout.write(json.dumps(obj, ensure_ascii=False) + "\n")
    sys.stdout.flush()


def main():
    worker = RapidOCRWorker()
    while True:
        line = sys.stdin.readline()
        if not line:
            return
        line = line.strip()
        if not line:
            continue

        req_id = ""
        try:
            req = json.loads(line)
            req_id = str(req.get("id") or "")
            req_type = str(req.get("type") or "ocr").strip().lower()

            if req_type == "ping":
                write_line({"id": req_id, "success": True})
                continue

            image_path = str(req.get("image_path") or "")
            profile = str(req.get("profile") or "default")
            debug = bool(req.get("debug") or False)
            out = worker.recognize(image_path=image_path, profile=profile, debug=debug)
            out["id"] = req_id
            write_line(out)
        except Exception as e:
            tb = traceback.format_exc()
            write_line({"id": req_id, "success": False, "error": str(e), "traceback": tb})


if __name__ == "__main__":
    main()

