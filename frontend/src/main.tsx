import React from 'react';
import ReactDOM from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter } from 'react-router-dom';
import App from './app/App';
import { CartProvider } from './features/cart/CartContext';
import './styles/tokens.css';
import './features/products/catalog.css';

const queryClient = new QueryClient();

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <CartProvider><App /></CartProvider>
      </BrowserRouter>
    </QueryClientProvider>
  </React.StrictMode>,
);
