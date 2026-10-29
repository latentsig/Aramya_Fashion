export interface Category {
  id: string;
  title: string;
  subtitle: string;
  image: string;
}

export interface Product {
  name: string;
  category: string;
  label: string;
  image: string;
  priceRupees: number;
}

export const categories: Category[] = [
  { id: 'sarees', title: 'Sarees', subtitle: 'A drape for every celebration', image: 'saree1.jpg' },
  { id: 'bridal', title: 'Bridal & Lehengas', subtitle: 'For the day you will always remember', image: 'bridal lehanga.jpg' },
  { id: 'occasion', title: 'Occasionwear', subtitle: 'Festive sets, gowns and shararas', image: 'sharara1.avif' },
  { id: 'menswear', title: 'Menswear', subtitle: 'Considered festive dressing', image: 'menblazer2.jpg' },
  { id: 'kids', title: 'Little Ones', subtitle: 'A little something for their big day', image: 'kidzgown2.jpg' },
];

export const products: Product[] = [
  { name: 'The Heirloom Saree', category: 'sarees', label: 'Sarees', image: 'saree1.jpg', priceRupees: 8900 },
  { name: 'Saanjh Silk Saree', category: 'sarees', label: 'Sarees', image: 'saree3.jpg', priceRupees: 12500 },
  { name: 'Rosewater Drapes', category: 'sarees', label: 'Sarees', image: 'saree2.jpg', priceRupees: 9800 },
  { name: 'Temple Garden Saree', category: 'sarees', label: 'Sarees', image: 'saree4.jpg', priceRupees: 14200 },
  { name: 'The Meera Edit', category: 'sarees', label: 'Sarees', image: 'saree5.jpg', priceRupees: 11900 },
  { name: 'Gulabi Lehenga', category: 'bridal', label: 'Bridal & lehengas', image: 'lehanga1.jpg', priceRupees: 28000 },
  { name: 'The Noor Edit', category: 'bridal', label: 'Bridal & lehengas', image: 'lehanga4.jpg', priceRupees: 34500 },
  { name: 'Gul Mahal Lehenga', category: 'bridal', label: 'Bridal & lehengas', image: 'lehanga2.jpg', priceRupees: 32000 },
  { name: 'Aabha Embroidered Lehenga', category: 'bridal', label: 'Bridal & lehengas', image: 'lehanga3.jpg', priceRupees: 29500 },
  { name: 'The Sunehri Set', category: 'bridal', label: 'Bridal & lehengas', image: 'lehanga5.jpg', priceRupees: 36000 },
  { name: 'Rani Sharara Set', category: 'occasion', label: 'Occasionwear', image: 'sharara1.avif', priceRupees: 16900 },
  { name: 'Evening in Rose', category: 'occasion', label: 'Occasionwear', image: 'gown2.webp', priceRupees: 19500 },
  { name: 'The Celebration Gown', category: 'occasion', label: 'Occasionwear', image: 'gown4.webp', priceRupees: 24000 },
  { name: 'Festive Half Saree', category: 'occasion', label: 'Occasionwear', image: 'halfsaree2.jpg', priceRupees: 15500 },
  { name: 'Mehfil Sharara', category: 'occasion', label: 'Occasionwear', image: 'sharara2.webp', priceRupees: 17500 },
  { name: 'Gulnaar Sharara Set', category: 'occasion', label: 'Occasionwear', image: 'sharara3.webp', priceRupees: 18900 },
  { name: 'The Noor Sharara', category: 'occasion', label: 'Occasionwear', image: 'sharara4.webp', priceRupees: 16500 },
  { name: 'Celebration in Bloom', category: 'occasion', label: 'Occasionwear', image: 'sharara5.webp', priceRupees: 20000 },
  { name: 'Soft Gold Gown', category: 'occasion', label: 'Occasionwear', image: 'gown3.webp', priceRupees: 22500 },
  { name: 'The Saanvi Gown', category: 'occasion', label: 'Occasionwear', image: 'gown6.webp', priceRupees: 21000 },
  { name: 'Phool Half Saree', category: 'occasion', label: 'Occasionwear', image: 'halfsaree1.jpg', priceRupees: 14500 },
  { name: 'Madhuri Half Saree', category: 'occasion', label: 'Occasionwear', image: 'halfsaree3.jpg', priceRupees: 16000 },
  { name: 'The Lilac Half Saree', category: 'occasion', label: 'Occasionwear', image: 'halfsaree4.jpg', priceRupees: 15900 },
  { name: 'Sundari Half Saree', category: 'occasion', label: 'Occasionwear', image: 'halfsaree5.jpg', priceRupees: 15500 },
  { name: 'Modern Bandhgala', category: 'menswear', label: 'Menswear', image: 'menblazer2.jpg', priceRupees: 18500 },
  { name: 'The Ivory Bandhgala', category: 'menswear', label: 'Menswear', image: 'menblazer1.jpg', priceRupees: 20500 },
  { name: 'Aramya Evening Blazer', category: 'menswear', label: 'Menswear', image: 'menblazer3.jpg', priceRupees: 17900 },
  { name: 'The Saffron Blazer', category: 'menswear', label: 'Menswear', image: 'menblazer4.jpg', priceRupees: 19500 },
  { name: 'Classic Celebration Blazer', category: 'menswear', label: 'Menswear', image: 'menblazer5.jpg', priceRupees: 18200 },
  { name: 'The Aramya Kurtha', category: 'menswear', label: 'Menswear', image: 'menkurtha3.jpg', priceRupees: 7900 },
  { name: 'The Festive Kurtha', category: 'menswear', label: 'Menswear', image: 'menkurtha1.jpg', priceRupees: 7500 },
  { name: 'Ivory Occasion Kurtha', category: 'menswear', label: 'Menswear', image: 'menkurtha2.jpg', priceRupees: 8200 },
  { name: 'The Heritage Kurtha', category: 'menswear', label: 'Menswear', image: 'menkurtha4.jpg', priceRupees: 8900 },
  { name: 'Sage Celebration Kurtha', category: 'menswear', label: 'Menswear', image: 'menkurtha5.jpg', priceRupees: 7900 },
  { name: 'Little Celebration', category: 'kids', label: 'Little ones', image: 'kidzgown2.jpg', priceRupees: 4500 },
  { name: 'Tiny Festive Set', category: 'kids', label: 'Little ones', image: 'kidzgown4.jpg', priceRupees: 5200 },
  { name: 'Little Rose Gown', category: 'kids', label: 'Little ones', image: 'kidzgown1.jpg', priceRupees: 4200 },
  { name: 'A Little Celebration', category: 'kids', label: 'Little ones', image: 'kidzgown3.jpg', priceRupees: 4800 },
  { name: 'Tiny Twirl Gown', category: 'kids', label: 'Little ones', image: 'kidzgown5.jpg', priceRupees: 5100 },
];

export function imageUrl(fileName: string): string {
  return `/media/${encodeURIComponent(fileName)}`;
}

export function formatPrice(priceRupees: number): string {
  return `₹ ${new Intl.NumberFormat('en-IN').format(priceRupees)}`;
}
