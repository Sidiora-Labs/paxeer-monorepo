import type { HTMLChakraProps } from '@chakra-ui/react';
import { chakra, Center } from '@chakra-ui/react';
import React from 'react';
import type { ChangeEvent, FormEvent, FocusEvent } from 'react';

import config from 'configs/app';
import useIsMobile from 'lib/hooks/useIsMobile';
import { useColorModeValue } from 'toolkit/chakra/color-mode';
import { Input } from 'toolkit/chakra/input';
import { InputGroup } from 'toolkit/chakra/input-group';
import { ClearButton } from 'toolkit/components/buttons/ClearButton';
import IconSvg from 'ui/shared/IconSvg';

const nameServicesFeature = config.features.nameServices;

interface Props extends Omit<HTMLChakraProps<'form'>, 'onChange'> {
  onChange?: (value: string) => void;
  onSubmit?: (event: FormEvent<HTMLFormElement>) => void;
  onBlur?: (event: FocusEvent<HTMLFormElement>) => void;
  onFocus?: () => void;
  onHide?: () => void;
  onClear?: () => void;
  onFormClick?: (event: React.MouseEvent<HTMLFormElement>) => void;
  isHeroBanner?: boolean;
  compact?: boolean;
  isSuggestOpen?: boolean;
  value?: string;
  readOnly?: boolean;
}

const SearchBarInput = (
  { onChange, onSubmit, isHeroBanner, compact, isSuggestOpen, onFocus, onBlur, onHide, onClear, onFormClick, value, readOnly, ...rest }: Props,
  ref: React.ForwardedRef<HTMLFormElement>,
) => {
  const innerRef = React.useRef<HTMLFormElement>(null);
  React.useImperativeHandle(ref, () => innerRef.current as HTMLFormElement, []);
  const isMobile = useIsMobile();
  const [ shortcut, setShortcut ] = React.useState('Ctrl K');

  React.useEffect(() => {
    setShortcut(/Mac|iPhone|iPad/.test(navigator.platform) ? '⌘K' : 'Ctrl K');
  }, []);

  const borderWidthHeroBanner = useColorModeValue(
    config.UI.homepage.heroBanner?.search?.border_width?.[0] ?? '1px',
    config.UI.homepage.heroBanner?.search?.border_width?.[1] ?? '1px',
  );

  const handleChange = React.useCallback((event: ChangeEvent<HTMLInputElement>) => {
    onChange?.(event.target.value);
  }, [ onChange ]);

  const handleKeyPress = React.useCallback((event: KeyboardEvent) => {
    if (isMobile) {
      return;
    }

    if (compact && (event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
      event.preventDefault();
      innerRef.current?.querySelector('input')?.focus();
      onFocus?.();
      return;
    }

    switch (event.key) {
      case '/': {
        if ([ 'INPUT', 'TEXTAREA' ].includes((event.target as HTMLElement).tagName)) {
          break;
        }

        if (!isSuggestOpen) {
          event.preventDefault();
          innerRef.current?.querySelector('input')?.focus();
          onFocus?.();
        }
        break;
      }
      case 'Escape': {
        if (isSuggestOpen) {
          innerRef.current?.querySelector('input')?.blur();
          onHide?.();
        }
        break;
      }
    }
  }, [ compact, isMobile, isSuggestOpen, onFocus, onHide ]);

  React.useEffect(() => {
    window.addEventListener('keydown', handleKeyPress);
    return () => {
      window.removeEventListener('keydown', handleKeyPress);
    };
  }, [ handleKeyPress ]);

  const getPlaceholder = () => {
    const clusterText = nameServicesFeature.isEnabled && nameServicesFeature.clusters.isEnabled ? ' / cluster ' : '';
    return `Search by address / txn hash / block / token${ clusterText }/... `;
  };

  const startElement = (
    <IconSvg
      name="search"
      boxSize={ 5 }
      mx={ 2 }
    />
  );

  const endElement = (
    <>
      <ClearButton onClick={ onClear } visible={ Boolean(value?.length) } mx={ 2 }/>
      { !isMobile && (
        <Center
          as="kbd"
          minW={ compact ? '42px' : '20px' }
          h="20px"
          fontSize="11px"
          color="text.muted"
          mr={ 2 }
          borderRadius="sm"
          borderWidth="1px"
          borderColor="input.element"
        >
          { compact ? shortcut : '/' }
        </Center>
      ) }
    </>
  );

  return (
    <chakra.form
      ref={ innerRef }
      noValidate
      onSubmit={ onSubmit }
      onBlur={ onBlur }
      onClick={ onFormClick }
      w="100%"
      backgroundColor={ compact ? 'raised' : 'bg.primary' }
      borderRadius={ compact ? '999px' : 'base' }
      position="relative"
      zIndex={ isSuggestOpen ? 'modal' : 'auto' }
      { ...rest }
    >
      <InputGroup
        startElement={ startElement }
        endElement={ endElement }
      >
        <Input
          h={ compact ? '36px' : undefined }
          borderRadius={ compact ? '999px' : 'base' }
          aria-label={ compact ? 'Search or ask' : undefined }
          size={{ base: isHeroBanner ? 'md' : 'sm', lg: 'md' }}
          placeholder={ compact ? 'Search or ask' : getPlaceholder() }
          value={ value }
          onChange={ handleChange }
          onFocus={ onFocus }
          tabIndex={ readOnly ? -1 : 0 }
          borderWidth={ isHeroBanner ? borderWidthHeroBanner : '1px' }
          borderStyle="solid"
          borderColor="input.border"
          color="input.fg"
          backgroundColor={ compact ? 'raised' : { base: isHeroBanner ? 'input.bg' : 'dialog.bg', lg: 'input.bg' } }
          _hover={{ borderColor: 'input.border.hover' }}
          _focusWithin={{ _placeholder: { color: 'input.placeholder' }, borderColor: 'input.border.focus', _hover: { borderColor: 'input.border.focus' } }}
          enterKeyHint="search"
        />
      </InputGroup>
    </chakra.form>
  );
};

export default React.memo(React.forwardRef(SearchBarInput));
