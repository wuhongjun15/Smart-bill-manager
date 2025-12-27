# Unified Dockerfile for Smart Bill Manager
# This builds both frontend and backend (Go) into a single image

# ============================================
# Stage 1: Build Frontend
# ============================================
FROM node:24-alpine AS frontend-builder

WORKDIR /app/frontend

# Copy frontend package files
COPY frontend/package*.json ./

# Install frontend dependencies
RUN npm ci

# Copy frontend source code
COPY frontend/ .

# Build the frontend (use npx to ensure local binaries are found)
RUN npx vue-tsc -b && npx vite build

# ============================================
# Stage 2: Build Backend (Go)
# ============================================
FROM golang:1.24 AS backend-builder

WORKDIR /app/backend

# Install build dependencies (CGO is required for sqlite)
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Copy go mod files
COPY backend-go/go.mod backend-go/go.sum ./

# Download dependencies
RUN go mod download

# Copy backend source code
COPY backend-go/ .

# Build the Go binary
RUN CGO_ENABLED=1 GOOS=linux go build -o server ./cmd/server

# ============================================
# Stage 3: Production Image
# ============================================
# Pin Debian base for reproducible apt packages (and Intel OpenCL runtime availability).
FROM nginx:stable-bookworm AS production

# Install runtime dependencies (Debian-based image).
# Note: onnxruntime wheels are built for glibc; Alpine (musl) often fails to install/build them.
RUN apt-get update && apt-get install -y --no-install-recommends \
    supervisor \
    ca-certificates \
    poppler-utils \
    python3 \
    python3-pip \
    python3-venv \
    python-is-python3 \
    libgomp1 \
    libstdc++6 \
    libgl1 \
    libglib2.0-0 \
    libtbb12 \
    ocl-icd-libopencl1 \
    && rm -rf /var/lib/apt/lists/*

# Optional Intel iGPU runtime for OpenVINO GPU plugin (UHD 630, etc).
# NOTE: This is disabled by default to keep CI builds stable and image size smaller.
ARG ENABLE_INTEL_GPU_RUNTIME=false
RUN if [ "$ENABLE_INTEL_GPU_RUNTIME" = "true" ]; then \
      set -eux; \
      ( \
        # Prefer Debian package (NEO OpenCL iGPU runtime). \
        apt-get update -o Acquire::Retries=3 && \
        apt-get install -y --no-install-recommends intel-opencl-icd clinfo \
      ) || ( \
        echo "Intel OpenCL runtime not available via apt; trying direct .deb install..." >&2; \
        rm -rf /var/lib/apt/lists/*; \
        apt-get update -o Acquire::Retries=3; \
        python3 -c "import urllib.request; urllib.request.urlretrieve('https://deb.debian.org/debian/pool/main/i/intel-compute-runtime/intel-opencl-icd_22.43.24595.41-1_amd64.deb','/tmp/intel-opencl-icd.deb')"; \
        echo \"698b08ffaec1da7821daad3965e78b1667a5e6f268fa7ccfaae24fb35c30dd08  /tmp/intel-opencl-icd.deb\" | sha256sum -c -; \
        dpkg -i /tmp/intel-opencl-icd.deb || apt-get -f install -y; \
        rm -f /tmp/intel-opencl-icd.deb; \
        apt-get install -y --no-install-recommends clinfo || true; \
      ) || ( \
        echo "WARNING: Failed to install Intel iGPU runtime in image; OpenVINO will run on CPU only." >&2; \
      ); \
      rm -rf /var/lib/apt/lists/* || true; \
    fi

# Install Python OCR dependencies (RapidOCR v3) in a virtualenv to avoid Debian PEP 668 restrictions.
RUN python3 -m venv /opt/venv
ENV VIRTUAL_ENV=/opt/venv
ENV PATH="/opt/venv/bin:${PATH}"
RUN ln -sf /opt/venv/bin/python3 /opt/venv/bin/python
RUN /opt/venv/bin/python3 -m pip install --no-cache-dir --upgrade pip setuptools wheel && \
    /opt/venv/bin/python3 -m pip install --no-cache-dir "rapidocr==3.*" onnxruntime && \
    /opt/venv/bin/python3 -m pip install --no-cache-dir "openvino==2025.4.1" "rapidocr-openvino==1.2.3" && \
    /opt/venv/bin/python3 -c "import rapidocr, onnxruntime; print('RapidOCR v3 OK')" && \
    /opt/venv/bin/python3 -c "import openvino, rapidocr_openvino; print('OpenVINO OCR OK')"

WORKDIR /app

# Copy built backend binary
COPY --from=backend-builder /app/backend/server ./backend/server

# Copy built frontend files to nginx html directory
COPY --from=frontend-builder /app/frontend/dist /usr/share/nginx/html

# Create necessary directories for backend
RUN mkdir -p /app/backend/uploads /app/backend/data /app/scripts

# Copy OCR scripts
COPY scripts/paddleocr_cli.py /app/scripts/

# Copy nginx configuration
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Create supervisord configuration
COPY supervisord.conf /etc/supervisord.conf

# Create log directories
RUN mkdir -p /var/log/supervisor

# Expose port 80 (nginx serves both frontend and proxies to backend)
EXPOSE 80

# Set environment variables
ENV PORT=3001

# Start supervisord which manages both nginx and Go backend
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisord.conf"]
