(function () {
  const info = window.LAN_DROP_INFO || { addresses: [] };
  const currentURL = `${window.location.origin}/`;
  const serverAddresses = info.addresses || [];
  const qrText = shouldUseLANAddress() ? serverAddresses[0] : currentURL;

  const elements = {
    qrImage: document.getElementById("qrImage"),
    currentAddress: document.getElementById("currentAddress"),
    addressList: document.getElementById("addressList"),
    fileInput: document.getElementById("fileInput"),
    folderInput: document.getElementById("folderInput"),
    dropZone: document.getElementById("dropZone"),
    progressText: document.getElementById("progressText"),
    progressPercent: document.getElementById("progressPercent"),
    progressBar: document.getElementById("progressBar"),
    fileList: document.getElementById("fileList"),
    fileCount: document.getElementById("fileCount"),
    emptyState: document.getElementById("emptyState"),
    refreshButton: document.getElementById("refreshButton"),
    statusPill: document.getElementById("statusPill"),
    previewModal: document.getElementById("previewModal"),
    previewTitle: document.getElementById("previewTitle"),
    previewBody: document.getElementById("previewBody"),
    previewOpen: document.getElementById("previewOpen")
  };

  const previewTypes = {
    image: new Set(["jpg", "jpeg", "png", "gif", "webp", "bmp", "svg", "ico", "avif"]),
    video: new Set(["mp4", "webm", "ogg", "ogv", "mov", "m4v"]),
    audio: new Set(["mp3", "wav", "ogg", "oga", "m4a", "flac", "aac"]),
    pdf: new Set(["pdf"]),
    text: new Set(["txt", "md", "json", "xml", "csv", "log", "yaml", "yml", "html", "css", "js", "ts", "go", "py", "java", "c", "cpp", "h", "hpp", "rs", "sh", "bat", "ps1", "ini", "conf"])
  };

  elements.currentAddress.textContent = qrText;
  elements.qrImage.src = `/qr?text=${encodeURIComponent(qrText)}`;
  renderAddresses();
  loadFiles();

  elements.fileInput.addEventListener("change", () => uploadFiles(Array.from(elements.fileInput.files || [])));
  elements.folderInput.addEventListener("change", () => uploadFiles(Array.from(elements.folderInput.files || [])));
  elements.refreshButton.addEventListener("click", loadFiles);
  elements.previewModal.addEventListener("click", (event) => {
    if (event.target.closest("[data-preview-close]")) {
      closePreview();
    }
  });
  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && !elements.previewModal.hidden) {
      closePreview();
    }
  });

  ["dragenter", "dragover"].forEach((eventName) => {
    elements.dropZone.addEventListener(eventName, (event) => {
      event.preventDefault();
      elements.dropZone.classList.add("dragging");
    });
  });
  ["dragleave", "drop"].forEach((eventName) => {
    elements.dropZone.addEventListener(eventName, (event) => {
      event.preventDefault();
      elements.dropZone.classList.remove("dragging");
    });
  });
  elements.dropZone.addEventListener("drop", (event) => {
    const files = Array.from(event.dataTransfer.files || []);
    uploadFiles(files);
  });

  function shouldUseLANAddress() {
    if (!serverAddresses.length) {
      return false;
    }
    const host = window.location.hostname.toLowerCase();
    return host === "localhost" || host === "::1" || host === "[::1]" || host.startsWith("127.");
  }

  function renderAddresses() {
    elements.addressList.innerHTML = "";
    const addresses = Array.from(new Set([qrText, currentURL, ...serverAddresses].filter(Boolean)));
    addresses.forEach((address) => {
      const link = document.createElement("a");
      link.className = "address-chip";
      link.href = address;
      link.textContent = address;
      elements.addressList.appendChild(link);
    });
  }

  async function loadFiles() {
    setStatus("读取中");
    try {
      const response = await fetch("/files");
      if (!response.ok) {
        throw new Error("读取文件列表失败");
      }
      const files = await response.json();
      renderFiles(files);
      setStatus("已连接");
    } catch (error) {
      setStatus("连接异常");
      elements.fileCount.textContent = error.message;
    }
  }

  function renderFiles(files) {
    elements.fileList.innerHTML = "";
    elements.emptyState.hidden = files.length !== 0;
    elements.fileCount.textContent = files.length ? `${files.length} 个项目` : "暂无文件";

    files.forEach((item) => {
      const row = document.createElement("article");
      row.className = "file-row";

      const main = document.createElement("div");
      main.className = "file-main";

      const name = document.createElement("div");
      name.className = "file-name";
      const icon = document.createElement("span");
      icon.className = "file-icon";
      icon.textContent = item.isDir ? "DIR" : "FILE";
      const pathText = document.createElement("span");
      pathText.className = "file-path";
      pathText.title = item.path;
      pathText.textContent = item.path;
      name.append(icon, pathText);

      const meta = document.createElement("div");
      meta.className = "file-meta";
      meta.textContent = `${item.isDir ? "文件夹，下载时自动打包 ZIP" : formatBytes(item.size)} · ${formatTime(item.modTime)}`;

      main.append(name, meta);

      const actions = document.createElement("div");
      actions.className = "file-actions";

      const previewType = getPreviewType(item.path);
      if (!item.isDir && previewType) {
        const preview = document.createElement("button");
        preview.className = "button small";
        preview.type = "button";
        preview.textContent = "预览";
        preview.addEventListener("click", () => openPreview(item, previewType));
        actions.appendChild(preview);
      }

      const download = document.createElement("a");
      download.className = "button small primary";
      download.href = `/download?path=${encodeURIComponent(item.path)}`;
      download.textContent = "下载";

      const remove = document.createElement("button");
      remove.className = "button small danger";
      remove.type = "button";
      remove.textContent = "删除";
      remove.addEventListener("click", () => deletePath(item.path, item.isDir));

      actions.append(download, remove);
      row.append(main, actions);
      elements.fileList.appendChild(row);
    });
  }

  function getPreviewType(filePath) {
    const ext = filePath.split(".").pop().toLowerCase();
    for (const [type, extensions] of Object.entries(previewTypes)) {
      if (extensions.has(ext)) {
        return type;
      }
    }
    return "";
  }

  function openPreview(item, type) {
    const url = `/preview?path=${encodeURIComponent(item.path)}`;
    elements.previewTitle.textContent = item.name || item.path;
    elements.previewOpen.href = url;
    elements.previewBody.innerHTML = "";

    if (type === "image") {
      const image = document.createElement("img");
      image.className = "preview-media";
      image.src = url;
      image.alt = item.name || item.path;
      elements.previewBody.appendChild(image);
    } else if (type === "video") {
      const video = document.createElement("video");
      video.className = "preview-media";
      video.src = url;
      video.controls = true;
      video.playsInline = true;
      elements.previewBody.appendChild(video);
    } else if (type === "audio") {
      const audio = document.createElement("audio");
      audio.className = "preview-audio";
      audio.src = url;
      audio.controls = true;
      elements.previewBody.appendChild(audio);
    } else if (type === "pdf") {
      const frame = document.createElement("iframe");
      frame.className = "preview-frame";
      frame.src = url;
      frame.title = item.name || item.path;
      elements.previewBody.appendChild(frame);
    } else {
      const pre = document.createElement("pre");
      pre.className = "preview-text";
      pre.textContent = "正在加载文本...";
      elements.previewBody.appendChild(pre);
      fetch(url)
        .then((response) => {
          if (!response.ok) {
            throw new Error("预览失败");
          }
          return response.text();
        })
        .then((text) => {
          pre.textContent = text;
        })
        .catch((error) => {
          pre.textContent = error.message;
        });
    }

    elements.previewModal.hidden = false;
    document.body.classList.add("modal-open");
  }

  function closePreview() {
    elements.previewModal.hidden = true;
    elements.previewBody.innerHTML = "";
    document.body.classList.remove("modal-open");
  }

  async function deletePath(path, isDir) {
    const label = isDir ? "文件夹" : "文件";
    if (!window.confirm(`确定删除这个${label}吗？\n${path}`)) {
      return;
    }
    const response = await fetch(`/delete?path=${encodeURIComponent(path)}`, { method: "DELETE" });
    const result = await response.json().catch(() => ({ ok: false, message: "删除失败" }));
    if (!response.ok || !result.ok) {
      window.alert(result.message || "删除失败");
      return;
    }
    await loadFiles();
  }

  async function uploadFiles(files) {
    if (!files.length) {
      return;
    }

    const totalBytes = files.reduce((sum, file) => sum + file.size, 0);
    let finishedBytes = 0;
    let uploaded = 0;

    setProgress(0, `准备上传 ${files.length} 个文件`);
    setStatus("上传中");

    try {
      for (const file of files) {
        const relativePath = file.webkitRelativePath || file.name;
        await uploadOne(file, relativePath, (loaded) => {
          const percent = totalBytes === 0 ? 100 : ((finishedBytes + loaded) / totalBytes) * 100;
          setProgress(percent, `正在上传 ${relativePath}`);
        });
        finishedBytes += file.size;
        uploaded += 1;
      }
      setProgress(100, `上传完成：${uploaded} 个文件`);
      setStatus("已连接");
      await loadFiles();
    } catch (error) {
      setStatus("上传失败");
      setProgress(totalBytes ? (finishedBytes / totalBytes) * 100 : 0, error.message);
      window.alert(error.message);
    } finally {
      elements.fileInput.value = "";
      elements.folderInput.value = "";
    }
  }

  function uploadOne(file, relativePath, onProgress) {
    return new Promise((resolve, reject) => {
      const form = new FormData();
      form.append("path", relativePath);
      form.append("file", file, file.name);

      const xhr = new XMLHttpRequest();
      xhr.open("POST", "/upload");
      xhr.upload.onprogress = (event) => {
        if (event.lengthComputable) {
          onProgress(event.loaded);
        }
      };
      xhr.onload = () => {
        const result = parseJSON(xhr.responseText);
        if (xhr.status >= 200 && xhr.status < 300 && result.ok) {
          resolve(result);
          return;
        }
        reject(new Error(result.message || `上传失败：HTTP ${xhr.status}`));
      };
      xhr.onerror = () => reject(new Error("网络错误，上传中断"));
      xhr.send(form);
    });
  }

  function parseJSON(text) {
    try {
      return JSON.parse(text);
    } catch (_) {
      return {};
    }
  }

  function setProgress(percent, text) {
    const value = Math.max(0, Math.min(100, percent || 0));
    elements.progressBar.style.width = `${value.toFixed(1)}%`;
    elements.progressPercent.textContent = `${Math.round(value)}%`;
    elements.progressText.textContent = text;
  }

  function setStatus(text) {
    elements.statusPill.textContent = text;
  }

  function formatBytes(bytes) {
    if (!bytes) {
      return "0 B";
    }
    const units = ["B", "KB", "MB", "GB", "TB"];
    const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
    const value = bytes / Math.pow(1024, index);
    return `${value.toFixed(value >= 10 || index === 0 ? 0 : 1)} ${units[index]}`;
  }

  function formatTime(value) {
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) {
      return "";
    }
    return date.toLocaleString();
  }
})();
