#!/usr/bin/env python3
"""
PDF Text Extract CLI (PyMuPDF)

Usage:
  python pdf_text_cli.py <pdf_path>

Outputs strict JSON only (required by Go backend parsing).
"""

from __future__ import annotations

import contextlib
import json
import os
import sys
from importlib import metadata


@contextlib.contextmanager
def suppress_child_output():
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


def main():
    if len(sys.argv) < 2 or not sys.argv[1]:
        print(json.dumps({"success": False, "error": "No PDF path provided"}))
        sys.exit(1)

    pdf_path = sys.argv[1]
    if not os.path.exists(pdf_path):
        print(json.dumps({"success": False, "error": f"PDF file not found: {pdf_path}"}))
        sys.exit(1)

    try:
        with suppress_child_output():
            import fitz  # PyMuPDF

            pymupdf_version = "unknown"
            try:
                pymupdf_version = metadata.version("pymupdf")
            except metadata.PackageNotFoundError:
                try:
                    pymupdf_version = metadata.version("PyMuPDF")
                except metadata.PackageNotFoundError:
                    pass

            doc = fitz.open(pdf_path)
            raw_parts = []
            ordered_parts = []
            page_count = 0

            for page in doc:
                page_count += 1
                try:
                    raw_parts.append(page.get_text("text") or "")
                except Exception:
                    raw_parts.append("")
                # Blocks with position, sorted by y then x
                try:
                    blocks = page.get_text("blocks") or []
                    blocks = sorted(blocks, key=lambda b: (b[1], b[0]))
                    for b in blocks:
                        if len(b) >= 5 and b[4]:
                            txt = str(b[4]).strip()
                            if txt:
                                ordered_parts.append(txt)
                except Exception:
                    # ignore this page ordering failure
                    continue

            doc.close()

        raw_text = "\n".join(t for t in raw_parts if t)
        ordered_text = "\n".join(ordered_parts)
        final_text = ordered_text or raw_text

        print(
            json.dumps(
                {
                    "success": True,
                    "text": final_text,
                    "raw_text": raw_text,
                    "ordered": bool(ordered_text),
                    "page_count": page_count,
                    "extractor": f"pymupdf-{pymupdf_version}",
                },
                ensure_ascii=False,
            )
        )
        return
    except ImportError:
        print(
            json.dumps(
                {"success": False, "error": "PyMuPDF not available. Install pymupdf."},
                ensure_ascii=False,
            )
        )
        sys.exit(1)
    except Exception as e:
        print(json.dumps({"success": False, "error": str(e)}, ensure_ascii=False))
        sys.exit(1)


if __name__ == "__main__":
    main()
