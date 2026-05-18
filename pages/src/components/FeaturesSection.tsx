import React from 'react';
import { useTranslation } from '../i18n';

const FeaturesSection: React.FC = () => {
  const { t } = useTranslation();

  const features = [
    {
      icon: 'fa-robot',
      color: 'text-brand-400',
      bgColor: 'bg-brand-500/10',
      title: t('features.feat1Title'),
      description: t('features.feat1Desc'),
    },
    {
      icon: 'fa-crosshairs',
      color: 'text-cyan-400',
      bgColor: 'bg-cyan-500/10',
      title: t('features.feat2Title'),
      description: t('features.feat2Desc'),
    },
    {
      icon: 'fa-link',
      color: 'text-purple-400',
      bgColor: 'bg-purple-500/10',
      title: t('features.feat3Title'),
      description: t('features.feat3Desc'),
    },
    {
      icon: 'fa-bolt',
      color: 'text-yellow-400',
      bgColor: 'bg-yellow-500/10',
      title: t('features.feat4Title'),
      description: t('features.feat4Desc'),
    },
    {
      icon: 'fa-brain',
      color: 'text-orange-400',
      bgColor: 'bg-orange-500/10',
      title: t('features.feat5Title'),
      description: t('features.feat5Desc'),
    },
    {
      icon: 'fa-scroll',
      color: 'text-pink-400',
      bgColor: 'bg-pink-500/10',
      title: t('features.feat6Title'),
      description: t('features.feat6Desc'),
    },
  ];

  return (
    <section id="features" className="py-24 relative noise-overlay">
      {/* Ambient glow */}
      <div className="absolute top-1/3 right-1/4 w-[500px] h-[500px] rounded-full bg-purple-500/[0.02] blur-[100px] pointer-events-none"></div>
      <div className="absolute bottom-1/3 left-1/4 w-[400px] h-[400px] rounded-full bg-cyan-500/[0.02] blur-[80px] pointer-events-none"></div>

      <div className="relative z-10">
        <div className="max-w-7xl mx-auto px-6">
          <div className="text-center mb-16">
            {/* Section pill badge */}
            <div className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full border border-slate-700/50 bg-dark-700/30 backdrop-blur-sm mb-6">
              <span className="text-xs text-slate-400 font-medium tracking-wide">{t('features.sectionBadge')}</span>
            </div>
            <h2 className="text-4xl font-bold text-white mb-4">
              {t('features.title')}
            </h2>
            <p className="text-slate-400 max-w-2xl mx-auto">
              {t('features.subtitle')}
            </p>
          </div>

          <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-6">
            {features.map((feature) => (
              <div key={feature.icon} className="feature-card rounded-2xl p-6 group glass">
                <div className={`w-12 h-12 rounded-xl ${feature.bgColor} flex items-center justify-center mb-4 group-hover:scale-110 transition-transform duration-300`}>
                  <i className={`fa-solid ${feature.icon} ${feature.color} text-lg`}></i>
                </div>
                <h3 className="text-white font-semibold text-lg mb-2">{feature.title}</h3>
                <p className="text-slate-400 text-sm leading-relaxed">{feature.description}</p>
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
};

export default FeaturesSection;
