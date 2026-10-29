import { Route, Routes } from 'react-router-dom';
import { SiteHeader } from '../components/layout/SiteHeader';
import { SiteFooter } from '../components/layout/SiteFooter';
import { HomePage } from './HomePage';
import { CollectionPage } from './CollectionPage';
import { CartPage } from './CartPage';
import { CheckoutPage } from './CheckoutPage';
import { AccountPage, LoginPage, RegisterPage } from './AuthPages';
import { AccountOrdersPage, OrderDetailPage } from './OrdersPages';
import { ProductPage } from './ProductPage';
import { StudioRequestDialog } from '../components/appointments/StudioRequestDialog';

export default function App() {
  return (
    <>
      <div className="announcement">Made for your moments. Crafted to be yours. <a href="#appointment">Book a studio appointment <span aria-hidden="true">↗</span></a></div>
      <SiteHeader />
      <Routes>
        <Route path="/" element={<HomePage />} />
        <Route path="/collections" element={<HomePage />} />
        <Route path="/collections/:slug" element={<CollectionPage />} />
        <Route path="/products/:slug" element={<ProductPage />} />
        <Route path="/cart" element={<CartPage />} />
        <Route path="/checkout" element={<CheckoutPage />} />
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />
        <Route path="/account" element={<AccountPage />} />
        <Route path="/account/orders" element={<AccountOrdersPage />} />
        <Route path="/account/orders/:id" element={<OrderDetailPage />} />
        <Route path="*" element={<main className="section-pad"><h1>Page not found</h1></main>} />
      </Routes>
      <SiteFooter />
      <StudioRequestDialog />
    </>
  );
}
