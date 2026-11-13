import type { HighlightsBannerConfig } from 'types/homepage';

export const IMAGE_URL_1 = 'https://localhost:3000/my-image.png';
export const IMAGE_URL_2 = 'https://localhost:3000/my-image-2.png';
export const IMAGE_URL_3 = 'https://localhost:3000/my-image-3.png';
export const HIGHLIGHTS_CONFIG_URL = 'https://localhost:3000/homepage-highlights-config.json';
export const HIGHLIGHTS_CONFIG: Array<HighlightsBannerConfig> = [
  // no adaptive
  {
    title: 'Duck Deep into Transactions',
    description: 'Explore and track all blockchain transactions',
    side_img_url: [ IMAGE_URL_1 ],
    title_color: [ '#D9FE41' ],
    description_color: [ '#CBD0D7' ],
    background: [ '#06331B' ],
    redirect_url: 'https://example.com',
    is_pinned: true,
  },
  // adaptive
  {
    title: 'Geese Token Swap',
    description: 'Swap tokens across different protocols',
    title_color: [ '#1e40af', '#93c5fd' ],
    description_color: [ '#64748b', '#94a3b8' ],
    background: [
      'linear-gradient(82.75deg, #FDDCEF 0.08%, #FAF5FB 51.54%, #FFFBDB 104.15%)',
      'linear-gradient(82.75deg, #4F2D6A 0.08%, #3D425A 51.54%, #3A6024 104.15%)',
    ],
    side_img_url: [ IMAGE_URL_2, IMAGE_URL_3 ],
    page_path: '/essential-dapps/swap',
    is_pinned: true,
  },
  // default
  {
    title: 'Duckling Smart Contracts',
    description: 'Discover newly deployed smart contracts',
    is_pinned: true,
  },
];
