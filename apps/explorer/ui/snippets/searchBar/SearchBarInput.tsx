import type { HTMLChakraProps } from '@chakra-ui/react';
import { chakra, Center } from '@chakra-ui/react';
import React from 'react';
import type { ChangeEvent, FormEvent, FocusEvent } from 'react';

import useIsMobile from 'lib/hooks/useIsMobile';
import { Input } from 'toolkit/chakra/input';
import { InputGroup } from 'toolkit/chakra/input-group';
import { ClearButton } from 'toolkit/components/buttons/ClearButton';
import IconSvg from 'ui/shared/IconSvg';

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
  const isHero = Boolean(isHeroBanner && !compact);
  const [ shortcut, setShortcut ] = React.useState('Ctrl K');

  React.useEffect(() => {
    setShortcut(/Mac|iPhone|iPad/.test(navigator.platform) ? '⌘K' : 'Ctrl K');
  }, []);

  const handleChange = React.useCallback((event: ChangeEvent<HTMLInputElement>) => {
    onChange?.(event.target.value);
  }, [ onChange ]);

  const handleKeyPress = React.useCallback((event: KeyboardEvent) => {
    if (isMobile) {
      return;
    }

    switch (event.key) {
      case 'Escape': {
        if (isSuggestOpen) {
          innerRef.current?.querySelector('input')?.blur();
          onHide?.();
        }
        break;
      }
    }
  }, [ isMobile, isSuggestOpen, onHide ]);

  React.useEffect(() => {
    window.addEventListener('keydown', handleKeyPress);
    return () => {
      window.removeEventListener('keydown', handleKeyPress);
    };
  }, [ handleKeyPress ]);

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
          minW="42px"
          h="20px"
          px={ 1 }
          fontSize="11px"
          color="text.muted"
          whiteSpace="nowrap"
          mr={ 2 }
          borderRadius="sm"
          borderWidth="1px"
          borderColor="border.strong"
        >
          { shortcut }
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
      w={ isHero ? '640px' : '320px' }
      maxW="100%"
      flexShrink={ 1 }
      backgroundColor="raised"
      borderRadius={ isHero ? '6px' : '999px' }
      position="relative"
      zIndex={ isSuggestOpen ? 'modal' : 'auto' }
      { ...rest }
    >
      <InputGroup
        startElement={ startElement }
        endElement={ endElement }
      >
        <Input
          size="md"
          h={ isHero ? '52px' : '36px' }
          borderRadius={ isHero ? '6px' : '999px' }
          aria-label={ compact ? 'Search or ask' : 'Search' }
          placeholder={ isHero ? 'Search or ask: a transaction, address, block or token' : 'Search or ask…' }
          value={ value }
          onChange={ handleChange }
          onFocus={ onFocus }
          tabIndex={ readOnly ? -1 : 0 }
          borderWidth="1px"
          borderStyle="solid"
          borderColor="border.strong"
          color="text.primary"
          backgroundColor="raised"
          _placeholder={{ color: 'text.muted' }}
          _placeholderShown={{ borderColor: 'border.strong' }}
          _hover={{ borderColor: 'border.strong' }}
          _focus={{ borderColor: 'accent', boxShadow: 'none', _hover: { borderColor: 'accent' } }}
          _focusVisible={{ outline: '2px solid', outlineColor: 'focusRing', outlineOffset: '2px' }}
          _readOnly={{ bg: 'raised', borderColor: 'border.strong' }}
          readOnly={ readOnly }
          enterKeyHint="search"
        />
      </InputGroup>
    </chakra.form>
  );
};

export default React.memo(React.forwardRef(SearchBarInput));
