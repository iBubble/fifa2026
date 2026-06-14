(function() {
  // 注入自定义样式的 CSS 规则，以支持淡入和缩放动画
  const style = document.createElement("style");
  style.innerHTML = `
    @keyframes alertFadeIn {
      from { opacity: 0; }
      to { opacity: 1; }
    }
    @keyframes alertScaleUp {
      from { transform: scale(0.92); }
      to { transform: scale(1); }
    }
  `;
  document.head.appendChild(style);

  // 1. 重写全局 window.alert
  window.alert = function(message) {
    let alertOverlay = document.getElementById("custom-alert-overlay");
    if (!alertOverlay) {
      alertOverlay = document.createElement("div");
      alertOverlay.id = "custom-alert-overlay";
      alertOverlay.style.cssText = `
        position: fixed;
        top: 0;
        left: 0;
        width: 100vw;
        height: 100vh;
        background: rgba(5, 8, 16, 0.7);
        backdrop-filter: blur(12px);
        -webkit-backdrop-filter: blur(12px);
        display: flex;
        align-items: center;
        justify-content: center;
        z-index: 10000;
        animation: alertFadeIn 0.25s forwards cubic-bezier(0.16, 1, 0.3, 1);
      `;

      const alertContent = document.createElement("div");
      alertContent.id = "custom-alert-content";
      alertContent.style.cssText = `
        background: rgba(20, 27, 45, 0.9);
        border: 1px solid var(--neon-green, #00ff88);
        box-shadow: 0 0 30px rgba(0, 255, 136, 0.15);
        border-radius: 14px;
        padding: 24px;
        max-width: 420px;
        width: 90%;
        text-align: center;
        animation: alertScaleUp 0.25s forwards cubic-bezier(0.16, 1, 0.3, 1);
        color: #f0f3f8;
      `;

      const title = document.createElement("div");
      title.innerHTML = "🤖 QUANT-AI 提示";
      title.style.cssText = `
        font-size: 15px;
        font-weight: 800;
        color: var(--neon-green, #00ff88);
        margin-bottom: 14px;
        letter-spacing: 0.5px;
      `;

      const msgText = document.createElement("div");
      msgText.id = "custom-alert-msg";
      msgText.style.cssText = `
        font-size: 13px;
        line-height: 1.6;
        margin-bottom: 20px;
        color: #8b9bb4;
      `;

      const confirmBtn = document.createElement("button");
      confirmBtn.id = "custom-alert-btn";
      confirmBtn.innerText = "确 认";
      confirmBtn.style.cssText = `
        height: 32px;
        padding: 0 28px;
        background: linear-gradient(135deg, var(--neon-purple, #8800ff), var(--neon-green, #00ff88));
        border: none;
        border-radius: 6px;
        color: white;
        font-weight: 800;
        font-size: 12px;
        cursor: pointer;
        outline: none;
        transition: all 0.2s;
        box-shadow: 0 4px 12px rgba(0, 255, 136, 0.15);
      `;

      confirmBtn.onmouseover = () => {
        confirmBtn.style.transform = "translateY(-1px)";
        confirmBtn.style.boxShadow = "0 6px 16px rgba(0, 255, 136, 0.35)";
      };
      confirmBtn.onmouseout = () => {
        confirmBtn.style.transform = "none";
        confirmBtn.style.boxShadow = "0 4px 12px rgba(0, 255, 136, 0.15)";
      };

      alertContent.appendChild(title);
      alertContent.appendChild(msgText);
      alertContent.appendChild(confirmBtn);
      alertOverlay.appendChild(alertContent);
      document.body.appendChild(alertOverlay);

      const closeAlert = () => {
        alertOverlay.style.display = "none";
      };
      confirmBtn.onclick = closeAlert;
      alertOverlay.onclick = (e) => {
        if (e.target === alertOverlay) closeAlert();
      };
    }

    document.getElementById("custom-alert-msg").innerHTML = message;
    alertOverlay.style.display = "flex";
  };

  // 2. 实现 window.customConfirm 异步确认框
  window.customConfirm = function(message) {
    return new Promise((resolve) => {
      let confirmOverlay = document.getElementById("custom-confirm-overlay");
      if (!confirmOverlay) {
        confirmOverlay = document.createElement("div");
        confirmOverlay.id = "custom-confirm-overlay";
        confirmOverlay.style.cssText = `
          position: fixed;
          top: 0;
          left: 0;
          width: 100vw;
          height: 100vh;
          background: rgba(5, 8, 16, 0.7);
          backdrop-filter: blur(12px);
          -webkit-backdrop-filter: blur(12px);
          display: flex;
          align-items: center;
          justify-content: center;
          z-index: 10000;
          animation: alertFadeIn 0.25s forwards cubic-bezier(0.16, 1, 0.3, 1);
        `;

        const confirmContent = document.createElement("div");
        confirmContent.id = "custom-confirm-content";
        confirmContent.style.cssText = `
          background: rgba(20, 27, 45, 0.9);
          border: 1px solid var(--neon-purple, #8800ff);
          box-shadow: 0 0 30px rgba(136, 0, 255, 0.15);
          border-radius: 14px;
          padding: 24px;
          max-width: 420px;
          width: 90%;
          text-align: center;
          animation: alertScaleUp 0.25s forwards cubic-bezier(0.16, 1, 0.3, 1);
          color: #f0f3f8;
        `;

        const title = document.createElement("div");
        title.innerHTML = "🤖 QUANT-AI 决策确认";
        title.style.cssText = `
          font-size: 15px;
          font-weight: 800;
          color: #c084fc;
          margin-bottom: 14px;
          letter-spacing: 0.5px;
        `;

        const msgText = document.createElement("div");
        msgText.id = "custom-confirm-msg";
        msgText.style.cssText = `
          font-size: 13px;
          line-height: 1.6;
          margin-bottom: 20px;
          color: #8b9bb4;
        `;

        const btnContainer = document.createElement("div");
        btnContainer.style.cssText = `
          display: flex;
          justify-content: center;
          gap: 16px;
        `;

        const cancelBtn = document.createElement("button");
        cancelBtn.innerText = "取 消";
        cancelBtn.style.cssText = `
          height: 32px;
          padding: 0 24px;
          background: rgba(255, 255, 255, 0.05);
          border: 1px solid rgba(255, 255, 255, 0.08);
          border-radius: 6px;
          color: #8b9bb4;
          font-weight: 800;
          font-size: 12px;
          cursor: pointer;
          outline: none;
          transition: all 0.2s;
        `;

        const okBtn = document.createElement("button");
        okBtn.innerText = "确 定";
        okBtn.style.cssText = `
          height: 32px;
          padding: 0 24px;
          background: linear-gradient(135deg, var(--neon-purple, #8800ff), var(--neon-green, #00ff88));
          border: none;
          border-radius: 6px;
          color: white;
          font-weight: 800;
          font-size: 12px;
          cursor: pointer;
          outline: none;
          transition: all 0.2s;
          box-shadow: 0 4px 12px rgba(136, 0, 255, 0.15);
        `;

        cancelBtn.onmouseover = () => {
          cancelBtn.style.background = "rgba(255, 255, 255, 0.1)";
          cancelBtn.style.color = "white";
        };
        cancelBtn.onmouseout = () => {
          cancelBtn.style.background = "rgba(255, 255, 255, 0.05)";
          cancelBtn.style.color = "#8b9bb4";
        };
        okBtn.onmouseover = () => {
          okBtn.style.transform = "translateY(-1px)";
          okBtn.style.boxShadow = "0 6px 16px rgba(136, 0, 255, 0.35)";
        };
        okBtn.onmouseout = () => {
          okBtn.style.transform = "none";
          okBtn.style.boxShadow = "0 4px 12px rgba(136, 0, 255, 0.15)";
        };

        btnContainer.appendChild(cancelBtn);
        btnContainer.appendChild(okBtn);
        confirmContent.appendChild(title);
        confirmContent.appendChild(msgText);
        confirmContent.appendChild(btnContainer);
        confirmOverlay.appendChild(confirmContent);
        document.body.appendChild(confirmOverlay);

        confirmOverlay.onclick = (e) => {
          if (e.target === confirmOverlay) {
            confirmOverlay.style.display = "none";
            resolve(false);
          }
        };
      }

      document.getElementById("custom-confirm-msg").innerHTML = message;
      confirmOverlay.style.display = "flex";

      const okBtn = confirmOverlay.querySelector("button:nth-child(2)");
      const cancelBtn = confirmOverlay.querySelector("button:nth-child(1)");

      okBtn.onclick = () => {
        confirmOverlay.style.display = "none";
        resolve(true);
      };
      cancelBtn.onclick = () => {
        confirmOverlay.style.display = "none";
        resolve(false);
      };
    });
  };
})();
