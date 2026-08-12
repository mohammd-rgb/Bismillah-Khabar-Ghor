import React, { useState, useEffect } from 'react';

// Codespaces環境・AWS EC2・ローカル実行環境に自動対応するAPIルート設定
const API_BASE = window.location.origin.includes('-3000.')
  ? window.location.origin.replace('-3000.', '-8080.')
  : (window.location.port === '3000' ? `${window.location.protocol}//${window.location.hostname}:8080` : '');

function App() {
  const [menu, setMenu] = useState([]);
  const [cart, setCart] = useState([]);
  const [orders, setOrders] = useState([]);
  const [tableNo, setTableNo] = useState(1);
  const [activeTab, setActiveTab] = useState('menu'); // 'menu', 'confirm', 'history'

  useEffect(() => {
    fetchMenu();
    fetchOrders();
  }, []);

  const fetchMenu = async () => {
    try {
      const res = await fetch(`${API_BASE}/api/menu`);
      if (res.ok) {
        const data = await res.json();
        setMenu(data || []);
      }
    } catch (e) {
      console.error("メニュー取得エラー:", e);
    }
  };

  const fetchOrders = async () => {
    try {
      const res = await fetch(`${API_BASE}/api/orders`);
      if (res.ok) {
        const data = await res.json();
        setOrders(data || []);
      }
    } catch (e) {
      console.error("注文履歴取得エラー:", e);
    }
  };

  const addToCart = (item) => {
    setCart((prevCart) => {
      const existing = prevCart.find((c) => c.id === item.id);
      if (existing) {
        return prevCart.map((c) =>
          c.id === item.id ? { ...c, quantity: c.quantity + 1 } : c
        );
      }
      return [...prevCart, { ...item, quantity: 1 }];
    });
  };

  const updateQuantity = (id, delta) => {
    setCart((prevCart) =>
      prevCart
        .map((item) => {
          if (item.id === id) {
            const newQty = item.quantity + delta;
            return newQty > 0 ? { ...item, quantity: newQty } : null;
          }
          return item;
        })
        .filter(Boolean)
    );
  };

  const calculateTotal = () => {
    return cart.reduce((sum, item) => sum + item.price * item.quantity, 0);
  };

  const handleOrderSubmit = async () => {
    if (cart.length === 0) return;

    const payload = {
      table_no: Number(tableNo),
      items: cart.map((item) => ({
        menu_item_id: item.id,
        quantity: item.quantity,
      })),
    };

    try {
      const res = await fetch(`${API_BASE}/api/orders`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });

      if (res.ok) {
        alert('注文が完了しました！');
        setCart([]);
        fetchOrders();
        setActiveTab('history');
      } else {
        alert('注文の送信に失敗しました。');
      }
    } catch (e) {
      console.error("注文エラー:", e);
    }
  };

  const handlePayment = async (orderId) => {
    if (!window.confirm('この注文の精算を行いますか？')) return;

    try {
      const res = await fetch(`${API_BASE}/api/orders/pay`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ order_id: orderId }),
      });

      if (res.ok) {
        alert('精算が完了しました。');
        fetchOrders();
      }
    } catch (e) {
      console.error("精算エラー:", e);
    }
  };

  return (
    <div className="app-container">
      <header className="navbar">
        <div className="nav-content">
          <h1 className="logo">Bismillah Khabar Ghor</h1>
          <div className="table-selector">
            <label>卓番号: </label>
            <input
              type="number"
              min="1"
              value={tableNo}
              onChange={(e) => setTableNo(e.target.value)}
            />
          </div>
        </div>
      </header>

      <div className="tab-bar">
        <button
          className={activeTab === 'menu' ? 'active' : ''}
          onClick={() => setActiveTab('menu')}
        >
          メニュー選択
        </button>
        <button
          className={activeTab === 'confirm' ? 'active' : ''}
          onClick={() => setActiveTab('confirm')}
        >
          注文確認 ({cart.reduce((a, b) => a + b.quantity, 0)})
        </button>
        <button
          className={activeTab === 'history' ? 'active' : ''}
          onClick={() => setActiveTab('history')}
        >
          注文履歴・精算
        </button>
      </div>

      <main className="main-layout">
        {activeTab === 'menu' && (
          <section className="menu-section">
            <h2>メニュー一覧</h2>
            <div className="menu-grid">
              {menu.map((item) => (
                <div key={item.id} className="card menu-card">
                  <div className="menu-info">
                    <span className="category">{item.category}</span>
                    <h3>{item.name}</h3>
                    <p className="description">{item.description}</p>
                    <span className="price">¥{item.price.toLocaleString()}</span>
                  </div>
                  <button
                    className="btn-primary"
                    onClick={() => addToCart(item)}
                  >
                    追加
                  </button>
                </div>
              ))}
            </div>
          </section>
        )}

        {activeTab === 'confirm' && (
          <section className="cart-section card">
            <h2>注文内容の確認 (卓番号: {tableNo})</h2>
            {cart.length === 0 ? (
              <p className="empty-message">カートに商品が入っていません。</p>
            ) : (
              <>
                <div className="cart-list">
                  {cart.map((item) => (
                    <div key={item.id} className="cart-item">
                      <div className="cart-item-details">
                        <h4>{item.name}</h4>
                        <span>¥{item.price.toLocaleString()}</span>
                      </div>
                      <div className="quantity-controls">
                        <button onClick={() => updateQuantity(item.id, -1)}>-</button>
                        <span>{item.quantity}</span>
                        <button onClick={() => updateQuantity(item.id, 1)}>+</button>
                      </div>
                      <span className="subtotal">
                        ¥{(item.price * item.quantity).toLocaleString()}
                      </span>
                    </div>
                  ))}
                </div>
                <div className="cart-summary">
                  <h3>合計: ¥{calculateTotal().toLocaleString()}</h3>
                  <button className="btn-submit" onClick={handleOrderSubmit}>
                    注文を確定する
                  </button>
                </div>
              </>
            )}
          </section>
        )}

        {activeTab === 'history' && (
          <section className="history-section">
            <h2>注文履歴・精算状況</h2>
            <div className="orders-list">
              {orders.map((order) => (
                <div key={order.id} className="card order-card">
                  <div className="order-header">
                    <div>
                      <span className="order-id">注文ID: #{order.id}</span>
                      <span className="table-badge">卓 {order.table_no}</span>
                    </div>
                    <span className={`status-badge ${order.status.toLowerCase()}`}>
                      {order.status === 'PAID' ? '精算済み' : '未精算'}
                    </span>
                  </div>
                  <div className="order-items-detail">
                    {order.details && order.details.map((d) => (
                      <div key={d.id} className="order-detail-row">
                        <span>{d.name} x {d.quantity}</span>
                        <span>¥{d.subtotal.toLocaleString()}</span>
                      </div>
                    ))}
                  </div>
                  <div className="order-footer">
                    <span className="order-total">
                      合計: ¥{order.total_price.toLocaleString()}
                    </span>
                    {order.status === 'UNPAID' && (
                      <button
                        className="btn-pay"
                        onClick={() => handlePayment(order.id)}
                      >
                        精算する
                      </button>
                    )}
                  </div>
                </div>
              ))}
              {orders.length === 0 && <p className="empty-message">注文履歴はありません。</p>}
            </div>
          </section>
        )}
      </main>
    </div>
  );
}

export default App;