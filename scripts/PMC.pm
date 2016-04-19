#!/usr/bin/perl

# apt-get install libnet-ip-perl  libnet-netmask-perl fping

package PMC; 
use Net::IP;
use Net::Netmask;
use Net::DNS;
use Beanstalk::Client;
use Data::Dumper ;

my $DHCP_MAPPINGS_FILE = '/var/lib/libvirt/dnsmasq/mappings/dhcp' ; 
my $HOSTS_MAPPINGS_DIR = '/var/lib/libvirt/dnsmasq/hostmappings' ; 
use strict; 


sub GetFreeIP {
	my $net = shift;
	my $range = GetRange($net)  ;
	my $ip = new Net::IP ($range) || die;
	# Loop
	my $free = 0 ; 
	my $valid_ip = 0 ; 
	do {
		print $ip->ip(), "\n";
		$valid_ip = $ip->ip() ; 
		$free = CheckIPFree($ip->ip()) ; 
	} while (++$ip && !$free);
	print "FreeIP = $valid_ip\n"; 
	return $valid_ip;
}

sub GetRange {

	my $net = shift ; 
	my $block = Net::Netmask->new($net);
	my $f = $block->nth(15);
	my $l = $block->nth( $block->size - 2 );
	return "$f - $l" 
}

sub CheckIPFree {
	my $ip = shift ; 
	my $ping_ok = CheckPing($ip) ; 
	print "PINGOK=$ping_ok\n"; 
	return 0 if($ping_ok) ; 
	my $dns_ok = CheckDNS($ip) ; 
	print "DNSOK = $dns_ok\n"; 
	return 0 if($dns_ok) ;
	return 1 ;

}

sub CheckPing{
	my $ip = shift ;
	my $out = `fping $ip`  ; 
	chomp $out ;
	print "OUT=$out\n"; 
	return 1 if($out =~ /alive/) ; 
	return 0 ;
}
	
sub CheckDNS {
	my $ip = shift;
	my $resolver = new Net::DNS::Resolver();
	my $packet = $resolver->query($ip) ; 
	if(defined $packet) {
		print STDERR __LINE__.": $ip is already defined in DNS\n"; 
		return 1 ; 
	}
	return 0; 
}

sub GenMAC {
	my $ip = shift ; 
	print "IP=$ip\n"; 
	my @octets = split/\./,$ip ; 
	my $mac = sprintf("52:54:00:%02x:%02x:%02x",$octets[1],$octets[2],$octets[3]);
	return $mac ; 
}


sub UpdateQ {

	my $server = shift ; 
	my $data = shift ;
	my $tube = shift ; 
	print "Connecting to $server , $tube\n" ;
	print Dumper $data;

	my $client = Beanstalk::Client->new( { server => $server, default_tube => $tube, }) or die "$!\n";

	my $job = $client->put( {}, 'host='.$data->{hostname},
	                            'ip='.$data->{ip},
				    'mac='. $data->{mac},
				    'cpu='.$data->{cpu},
				    'mem='.$data->{mem},
				    'ct='.$data->{ct}
				    ) ; 
}
sub GetMemory {
	my $sz = shift ; 
	my $mem = $1 if($sz =~ /M(\d+)/) ; 
	return $mem * 1024 * 1024 ; 
}
sub GetCPU {
	my $sz = shift ; 
	my $cpu = $1 if($sz =~ /C(\d+)/) ; 
	return $cpu; 
}
sub VerifyValidSize {
	my $s = shift; 
	print STDERR "Verify $s\n";
	my %approved;
	$approved{C} = 'CPU'; 
	$approved{M} = 'MEMORY'; 
	$approved{D} = 'DISK'; 
	my @fields = ($s =~ /([a-z]\d+)/gi) ; 
	return 0 if(! @fields) ;
	foreach (@fields) {
		my ($k,$v) = ($1,$2) if(/^([a-z])(\d+)$/i) ;
		return 0 if(!defined $approved{uc($k)}) ; 
	}
	return 1;
}
sub GenerateNetworkConfig { 
	my $hn =  shift; 
	my $ip = shift ; 
	my $mac = shift; 
	print "IP=$ip, mac = $mac, H=$hn\n";
	print "OPENING > ${HOSTS_MAPPINGS_DIR}/${ip}\n" ;
	open F,"> ${HOSTS_MAPPINGS_DIR}/${ip}" ;
	print F "$ip $hn\n"; 
	close F; 
	UpdateDHCPConf($mac,$ip,$hn,'ADD') ; 
	my $reload = `pkill dnsmasq && /usr/sbin/dnsmasq -C /etc/default/dnsmasq.conf` ; 
	print Dumper $reload; 
} 

sub UpdateDHCPConf {
	my $mac = shift ; 
	my $ip = shift ; 
	my $hn = shift ; 
	open F,$DHCP_MAPPINGS_FILE; 
	my @dhcpc = <F>; 
	close F; 
	my %dhcp_hash ; 
	foreach (@dhcpc) {
		chomp; 
		my ($mac1,@rest) = split/,/; 
		next if($mac !~ /(\w\w:){5,6}/) ; 
		$dhcp_hash{$mac1} = join(",",@rest) ; 
	}
	$dhcp_hash{$mac} = join(",",$ip,$hn) ; 
	open F,"> $DHCP_MAPPINGS_FILE" or die "$!\n" ; 
	foreach my $maci (sort keys %dhcp_hash) {
		print F join(",",$maci,$dhcp_hash{$maci}) . "\n"  ; 
	}
	close F; 
}
1;
