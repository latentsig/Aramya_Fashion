import React from 'react';
import ReactDOM from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter } from 'react-router-dom';
import App from './app/App';
import { CartProvider } from './features/cart/CartContext';
import { AuthProvider } from './features/auth/AuthContext';
import './styles/tokens.css';
import './styles/hero-video.css';
import './styles/pwa.css';
import './features/products/catalog.css';
import './features/products/product-detail.css';
import './features/auth/auth.css';
import './features/orders/orders.css';
import './features/appointments/appointments.css';

const queryClient = new QueryClient();

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <BrowserRouter>
          <CartProvider><App /></CartProvider>
        </BrowserRouter>
      </AuthProvider>
    </QueryClientProvider>
  </React.StrictMode>,
);
