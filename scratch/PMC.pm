#!/usr/bin/perl

package PMC; 

use strict; 
use XML::Simple; 
use Data::Dumper; 
use Data::Validate::IP qw (is_private_ipv4) ; 
use Net::DNS ; 
use Net::Ping ; 
use YAML::Syck; 


my $PMCDIR = '/var/local/pmc'; 
my $HOST_IMAGE_LOCATION = '/var/lib/libvirt/images'; 
my $ORIGINAL_SOURCE_IMAGE = '/var/lib/libvirt/images/trusty.ORIG.img'; 
my $GATEWAY = '192.168.200.1' ; 
my $LIBVIRT_XML_TEMPLATE = '/etc/default/PMCLibvirtTemplate.xml' ; 
my $CLOUD_INIT_DEF = '/etc/default/cloud-init'; 
my $FQDN  = 'phonepe.int' ;
my $CLOUD_LOCAL_DS = '/usr/bin/cloud-localds'; 
my $DHCP_MAPPINGS_FILE = '/var/lib/libvirt/dnsmasq/mappings/dhcp' ; 
my $HOSTS_MAPPINGS_DIR = '/var/lib/libvirt/dnsmasq/hostmappings' ; 
   
sub DefineAndStartVM {
	my $hn = shift ; 
	my $dnsf = `virsh define ${PMCDIR}/${hn}.xml && virsh start $hn`; 
	print $dnsf; 
}

sub VerifyValidSize {
	my $s = shift; 
	my %approved;
	$approved{C} = 'CPU'; 
	$approved{M} = 'MEMORY'; 
	$approved{D} = 'DISK'; 
	my @fields = ($s =~ /([a-z]\d+)/gi) ; 
	foreach (@fields) {
		my ($k,$v) = ($1,$2) if(/^([a-z])(\d+)$/i) ;
		return 0 if(!defined $approved{uc($k)}) ; 
	}
	return 1;
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
sub GetImageLocation {
	my $hn = shift; 
	my $image_loc = $HOST_IMAGE_LOCATION."/${hn}.img" ; 
	my $cp_st = system("cp",$ORIGINAL_SOURCE_IMAGE,$image_loc); 
	if(!$cp_st) {
		return $image_loc; 
	}
	die "Copy of $ORIGINAL_SOURCE_IMAGE -> $image_loc FAILED\n"; 
}

sub GenerateLibvirtXML{
	my $hn = shift ;
	my $ip = shift; 
	my $sz = shift; 
	my %ret; 
	$ret{PMC__HOSTNAME} = $hn;
	$ret{PMC__UUID} = GetUUID();
	$ret{PMC__MEMORY} = GetMemory($sz) ; 
	$ret{PMC__VCPU} = GetCPU($sz) ; 
	$ret{PMC__KERNEL} = $PMCDIR.'/trusty-server-cloudimg-amd64-vmlinuz-generic' ;
	$ret{PMC__CLOUDINITIMAGE} = "${PMCDIR}/hostdata.${hn}.img";
	$ret{PMC__MAC} = GenMAC($ip) ;
	$ret{PMC__IP} = $ip;
	$ret{PMC__GATEWAY} = $GATEWAY; 
	$ret{PMC__HOSTIMAGE} = GetImageLocation($hn) ; 
	open F,$LIBVIRT_XML_TEMPLATE ; 
	my @xml = <F> ;
	close F; 
	my $xml_str = join("",@xml) ; 
	foreach my $k (sort keys %ret) {
		$xml_str =~ s/__${k}__/$ret{$k}/g ; 
	}
	open F,"> ${PMCDIR}/${hn}.xml"; 
	print F $xml_str;
	close F; 

}
sub GetUUID {
	my $uuid = `uuidgen`; 
	chomp $uuid; 
	return $uuid; 
}

sub GenerateCloudInitConfig {
	my %ret; 
	my $hn = shift; 
	my $def = LoadFile($CLOUD_INIT_DEF) ; 
	#my $def = LoadFile("/home/user/my-user-data") ; 
	$def->{hostname} = $hn; 
	$def->{fqdn} = join(".",$hn,$FQDN) ; 
	#print Dumper $def; 
	my $yml = Dump($def) ; 
	$yml =~ s/^---/#cloud-config/; 
	open F,"> ${PMCDIR}/hostdata.${hn}" ;
	print F $yml;
	close F; 
	unlink "${PMCDIR}/hostdata.${hn}.img" ; 
	my $cloud_local_ds_f = system($CLOUD_LOCAL_DS,"${PMCDIR}/hostdata.${hn}.img","${PMCDIR}/hostdata.${hn}") ; 
	print "system($CLOUD_LOCAL_DS ${PMCDIR}/hostdata.${hn}.img ${PMCDIR}/hostdata.${hn}\n"; 
	print "STATUS=$cloud_local_ds_f\n"; 
	die if( $cloud_local_ds_f ) ; 
}


sub GenerateNetworkConfig { 
	my $hn =  shift; 
	my $ip = shift ; 
	my $f = shift ; 
	my $mac = GenMAC($ip) ; 
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


sub GenMAC {
	my $ip = shift ; 
	my @octets = split/\./,$ip ; 
	my $mac = sprintf("52:54:00:%02x:%02x:%02x",$octets[1],$octets[2],$octets[3]);
	return $mac ; 
}

sub VerifyValidIP {
	my $ip = shift; 
	if(! is_private_ipv4($ip)) {
		print STDERR __LINE__.": $ip: Invalid IP\n"; 
		return 0; 
	}
	if( CheckPing($ip)) {
		print STDERR __LINE__.": $ip pings\n"; 
		return 0 ;
	} 
	if( CheckDNS($ip) ) {
		print STDERR __LINE__.": $ip resolves\n"; 
		return 0 ;
	}
	return 1; 

}

sub CheckPing {
	my $ip = shift;
	my $p = Net::Ping->new(); 
	if ($p->ping($ip))  {
		print STDERR __LINE__.": $ip is pingable\n"; 
		$p->close();
		return 1  ; 
	}
	$p->close();
	return 0  ; 
}

sub VerifyValidHost {
	my $h = shift; 
	my $resolver = new Net::DNS::Resolver();
	my $packet = $resolver->query($h) ; 
	if(defined $packet) {
		print STDERR __LINE__.": $h is already defined in DNS\n"; 
		return 0 ; 
	}
	return 1; 
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

sub VerifyValidInputForDelete {
	my $hn = shift ; 
	my $ipf = shift; 
	my $ipval = `host $hn` ; 
	chomp $ipval; 
	$ipval = (split/\s+/,$ipval)[-1];
	die "$ipval: BAD value\n" if($ipval !~ /(\d+\.){3,3}\d+/) ; 
	if($ipval ne $ipf ) {
		print STDERR __LINE__.": $hn resolves to $ipval ( not $ipf ) \n"; 
		return 0; 
	}
	return 1;
}

sub DeleteAndDestroyVM {
	my $hn = shift ; 
	my $dnsf = `virsh destroy $hn  ; virsh undefine $hn`; 
	print $dnsf; 
}

sub DeleteHostImages {
	my $hn = shift; 
	unlink $HOST_IMAGE_LOCATION."/${hn}.img" ; 
	unlink "${PMCDIR}/hostdata.${hn}" ;
	unlink "${PMCDIR}/hostdata.${hn}.img" ;
	unlink "${PMCDIR}/${hn}.xml"; 
}

sub DeleteHostDnsDhcp {
	my $h = shift ; 
	my $ip = shift; 
	open F, $DHCP_MAPPINGS_FILE ;
	my @dhcpc = <F>; 
	close F; 
	my %dhcp_hash ; 
	my $found_match; 
	foreach (@dhcpc) {
		chomp; 
		s/\s+//g; 
		my ($mac1,$ipv,$hnv) = split/,/; 
		next if($mac1 !~ /(\w\w:){5,6}/) ; 
		if($hnv eq $h && $ip eq $ipv) {
			print STDERR "Deleting: $mac1, $hnv, $ipv\n"; 
			$found_match = 1; 
		} else {
			$dhcp_hash{$mac1} = join(",",$ipv,$hnv) ;
		}
	}
	if($found_match ) {
		open F,"> $DHCP_MAPPINGS_FILE" or die "$!\n" ; 
		foreach my $maci (sort keys %dhcp_hash) {
			print F join(",",$maci,$dhcp_hash{$maci}) . "\n"  ; 
		}
		close F; 
		unlink "${HOSTS_MAPPINGS_DIR}/${ip}" ;
	} else {
		print STDERR __LINE__.": $h, $ip, NOT DELETED, not found in config\n"; 
	}
}
