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
    statusPill: document.getElementById("statusPill")
  };

  elements.currentAddress.textContent = qrText;
  elements.qrImage.src = `/qr?text=${encodeURIComponent(qrText)}`;
  renderAddresses();
  loadFiles();

  elements.fileInput.addEventListener("change", () => uploadFiles(Array.from(elements.fileInput.files || [])));
  elements.folderInput.addEventListener("change", () => uploadFiles(Array.from(elements.folderInput.files || [])));
  elements.refreshButton.addEventListener("click", loadFiles);

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
