import { render } from '@testing-library/react';
import React from 'react';
import { ReCAPTCHA } from 'react-google-recaptcha';

export default function renderReCaptcha(ref: React.RefObject<ReCAPTCHA | null>) {
  return render(<ReCAPTCHA ref={ ref } sitekey="transcript-site-key" size="invisible"/>);
}
