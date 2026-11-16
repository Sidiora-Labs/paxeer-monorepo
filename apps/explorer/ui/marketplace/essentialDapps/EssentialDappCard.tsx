import { Text } from '@chakra-ui/react';

import { route } from 'nextjs-routes';

import { Button } from 'toolkit/chakra/button';
import { useColorModeValue } from 'toolkit/chakra/color-mode';
import { Image } from 'toolkit/chakra/image';
import { LinkOverlay, LinkBox } from 'toolkit/chakra/link';

type Props = {
  id: string;
  title: string;
  description: string;
  buttonText: string;
  imageUrl: string;
  darkImageUrl: string;
};

const EssentialDappCard = ({ id, title, description, buttonText, imageUrl, darkImageUrl }: Props) => {
  const imageSrc = useColorModeValue(imageUrl, darkImageUrl);

  return (
    <LinkBox
      as="article"
      containerType="inline-size"
      display="flex"
      flexDirection="column"
      alignItems="flex-start"
      flex="1 1 var(--chakra-spacing-40)"
      minW={ 0 }
      maxW={{ base: 'full', md: 'calc(var(--chakra-spacing-1) * 50)' }}
      p={{ base: 3, md: 5 }}
      border="var(--chakra-border-widths-1) solid"
      borderColor="hairline"
      borderRadius="base"
      _hover={{ boxShadow: 'md' }}
      _focusWithin={{ boxShadow: 'md' }}
      className="group"
    >
      <Image
        src={ imageSrc }
        alt={ title }
        w="var(--chakra-spacing-12)"
        objectFit="contain"
        h={{ base: 'calc(var(--chakra-spacing-1) * 9.25)', md: 'calc(var(--chakra-spacing-1) * 12.5)' }}
        mb={ 6 }
      />
      <Text textStyle={{ base: 'sm', md: 'xl' }} fontWeight="semibold" mb={ 2 }>{ title }</Text>
      <Text textStyle={{ base: 'xs', md: 'sm' }} mb={ 3 }>{ description }</Text>
      <Button asChild size="sm" variant="outline" mt="auto" w="full" whiteSpace="normal" h="auto" minH="touchTarget">
        <LinkOverlay
          href={ route({ pathname: '/essential-dapps/[id]', query: { id } }) }
          w={{ base: 'full', md: 'auto' }}
          mt="auto"
        >
          { buttonText }
        </LinkOverlay>
      </Button>
    </LinkBox>
  );
};

export default EssentialDappCard;
