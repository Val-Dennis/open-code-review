import React, { useEffect } from 'react';
import { useLocation } from 'react-router-dom';
import Navbar from './Navbar';
import HeroSection from './HeroSection';
import WhySection from './WhySection';
import FeaturesSection from './FeaturesSection';
import BenchmarkSection from './BenchmarkSection';
import QuickStartSection from './QuickStartSection';

const LandingPage: React.FC = () => {
  const location = useLocation();

  useEffect(() => {
    const scrollTo = (location.state as { scrollTo?: string })?.scrollTo;
    if (scrollTo) {
      const el = document.getElementById(scrollTo);
      if (el) {
        setTimeout(() => el.scrollIntoView({ behavior: 'smooth' }), 100);
      }
      window.history.replaceState({}, '');
    }
  }, [location.state]);

  return (
    <div className="min-h-screen bg-dark-900 noise-overlay">
      <Navbar />
      <HeroSection />
      <WhySection />
      <FeaturesSection />
      <BenchmarkSection />
      <QuickStartSection />
    </div>
  );
};

export default LandingPage;
