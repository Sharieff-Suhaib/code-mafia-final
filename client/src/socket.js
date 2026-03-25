/**
 * Native WebSocket client for the Go backend (GET /ws?username=&token=).
 * Wire format: { "type": string, "payload": any }
 */
class WebSocketClient {
  constructor() {
    this.ws = null;
    this.listeners = {};
    this._auth = {};
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = 5;
    this.reconnectDelay = 3000;
  }

  set auth(v) {
    this._auth = v || {};
  }

  get auth() {
    return this._auth;
  }

  connect() {
    const base = (process.env.REACT_APP_SERVER_BASEAPI || "")
      .replace("/api", "")
      .replace(/\/$/, "");
    if (!base) {
      console.error("REACT_APP_SERVER_BASEAPI is not set");
      return;
    }
    const token = localStorage.getItem("token");
    if (!token) {
      console.error("No token found, cannot connect to WebSocket");
      return;
    }
    const username = this._auth.username || "";
    const wsBase = base.replace(/^http/i, (m) =>
      m.toLowerCase() === "https" ? "wss" : "ws"
    );
    const url = `${wsBase}/ws?username=${encodeURIComponent(username)}&token=${encodeURIComponent(token)}`;

    this.ws = new WebSocket(url);

    this.ws.onopen = () => {
      console.log("WebSocket connected");
      this.reconnectAttempts = 0;
      this._emitLocal("connect");
    };

    this.ws.onmessage = (event) => {
      let data;
      try {
        data = JSON.parse(event.data);
      } catch (err) {
        console.error("WebSocket message parse error:", err);
        return;
      }

      const { type, payload } = data;
      try {
        if (type && this.listeners[type]) {
          this.listeners[type].forEach((cb) => cb(payload));
        }
        if (this.listeners["*"]) {
          this.listeners["*"].forEach((cb) => cb(type, payload));
        }
      } catch (err) {
        console.error("WebSocket listener error:", err);
      }
    };

    this.ws.onerror = (err) => {
      console.error("WebSocket error:", err);
    };

    this.ws.onclose = () => {
      console.log("WebSocket disconnected");
      this._emitLocal("disconnect");
      this.attemptReconnect();
    };
  }

  _emitLocal(event, data) {
    if (this.listeners[event]) {
      this.listeners[event].forEach((cb) => cb(data));
    }
  }

  attemptReconnect() {
    if (this.reconnectAttempts < this.maxReconnectAttempts) {
      this.reconnectAttempts++;
      console.log(
        `Attempting to reconnect (${this.reconnectAttempts}/${this.maxReconnectAttempts})...`
      );
      setTimeout(() => this.connect(), this.reconnectDelay);
    }
  }

  disconnect() {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  on(event, callback) {
    if (!this.listeners[event]) {
      this.listeners[event] = [];
    }
    this.listeners[event].push(callback);
  }

  onAny(callback) {
    this.on("*", callback);
  }

  off(event, callback) {
    if (!this.listeners[event]) {
      return;
    }
    if (callback) {
      this.listeners[event] = this.listeners[event].filter((cb) => cb !== callback);
    } else {
      delete this.listeners[event];
    }
  }

  emit(type, payload) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      const body =
        payload === undefined ? { type } : { type, payload };
      this.ws.send(JSON.stringify(body));
    } else {
      console.error("WebSocket is not connected");
    }
  }
}

const socket = new WebSocketClient();

export default socket;
